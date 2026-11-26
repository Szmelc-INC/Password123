// Package validate implements the fail-closed pre-commit and CI checks
// described in ARCHITECTURE.md section 8.4. Every error finding must stop a
// build; warnings are reported but do not fail.
package validate

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"filippo.io/age"
	"github.com/Szmelc-INC/Password123/internal/manifest"
	"github.com/Szmelc-INC/Password123/internal/vault"
	"github.com/Szmelc-INC/Password123/internal/vectors"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

// Level of a finding.
type Level string

const (
	Error Level = "error"
	Warn  Level = "warn"
	OK    Level = "ok"
)

// Finding is one check result.
type Finding struct {
	Check   string `json:"check"`
	Level   Level  `json:"level"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

// Options control which checks run.
type Options struct {
	SkipVectors bool
	// ExtraScanDirs are additional directories (for example a built _site)
	// scanned for private key material.
	ExtraScanDirs []string
}

// DemoIdentityPath is the only file allowed to contain a private identity.
const DemoIdentityPath = "demo/demo-identity.txt"

type run struct {
	root     string
	findings []Finding
	schemas  map[string]*jsonschema.Schema
}

func (r *run) add(check string, lvl Level, p, format string, a ...any) {
	r.findings = append(r.findings, Finding{Check: check, Level: lvl, Path: p, Message: fmt.Sprintf(format, a...)})
}

// Run executes every check and returns the findings.
func Run(root string, opt Options) []Finding {
	r := &run{root: root, schemas: map[string]*jsonschema.Schema{}}
	r.loadSchemas()
	lessons := r.checkLessons()
	r.checkVectorFiles(lessons, opt)
	r.checkPayloads(lessons)
	r.checkDemo(lessons)
	r.checkRecipients()
	r.checkSOPS()
	r.checkPrivateMaterial(opt.ExtraScanDirs)
	r.checkTrackedPaths()
	r.checkPortal()
	return r.findings
}

// Failed reports whether any finding is an error.
func Failed(fs []Finding) bool {
	for _, f := range fs {
		if f.Level == Error {
			return true
		}
	}
	return false
}

func (r *run) loadSchemas() {
	c := jsonschema.NewCompiler()
	for _, name := range []string{"lesson", "vectors", "payload", "demo"} {
		p := filepath.Join(r.root, "schemas", name+".schema.json")
		s, err := c.Compile(p)
		if err != nil {
			r.add("schema", Error, "schemas/"+name+".schema.json", "cannot compile schema: %v", err)
			continue
		}
		r.schemas[name] = s
	}
}

// ValidateYAML validates a YAML file against a compiled schema.
func (r *run) validateYAML(schema, rel string) bool {
	s := r.schemas[schema]
	if s == nil {
		return false
	}
	b, err := os.ReadFile(filepath.Join(r.root, rel))
	if err != nil {
		r.add("schema", Error, rel, "%v", err)
		return false
	}
	var doc any
	if err := yaml.Unmarshal(b, &doc); err != nil {
		r.add("schema", Error, rel, "invalid YAML: %v", err)
		return false
	}
	j, err := json.Marshal(doc)
	if err != nil {
		r.add("schema", Error, rel, "cannot convert YAML to JSON: %v", err)
		return false
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(j))
	if err != nil {
		r.add("schema", Error, rel, "%v", err)
		return false
	}
	if err := s.Validate(inst); err != nil {
		r.add("schema", Error, rel, "does not match %s schema: %v", schema, oneLine(err.Error()))
		return false
	}
	return true
}

// ValidatePayloadJSON validates decrypted or to-be-encrypted payload content.
func ValidatePayloadJSON(root string, b []byte) error {
	c := jsonschema.NewCompiler()
	s, err := c.Compile(filepath.Join(root, "schemas", "payload.schema.json"))
	if err != nil {
		return err
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("payload is not valid JSON: %w", err)
	}
	if err := s.Validate(inst); err != nil {
		return fmt.Errorf("payload does not match payload.schema.json: %s", oneLine(err.Error()))
	}
	return nil
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func (r *run) checkLessons() []manifest.Lesson {
	paths, err := manifest.ManifestPaths(r.root)
	if err != nil || len(paths) == 0 {
		r.add("manifest", Error, "curriculum/", "no lesson manifests found: %v", err)
		return nil
	}
	ok := true
	for _, p := range paths {
		ok = r.validateYAML("lesson", p) && ok
	}
	if !ok {
		return nil
	}
	lessons, err := manifest.LoadLessons(r.root)
	if err != nil {
		r.add("manifest", Error, "", "%v", err)
		return nil
	}
	seen := map[string]string{}
	for _, l := range lessons {
		if prev, dup := seen[l.ID]; dup {
			r.add("manifest", Error, l.File, "duplicate lesson id %s (also in %s)", l.ID, prev)
		}
		seen[l.ID] = l.File
		if !strings.HasPrefix(l.ID, l.Module[:3]) {
			r.add("manifest", Error, l.File, "lesson id %s does not start with module prefix %s", l.ID, l.Module[:3])
		}
		wantFile := "curriculum/" + l.Module + "/" + strings.TrimPrefix(l.ID, l.Module[:3]) + ".yaml"
		if l.File != wantFile {
			r.add("manifest", Error, l.File, "lesson %s must live in %s", l.ID, wantFile)
		}
		has := map[string]bool{}
		for _, lb := range l.Labels {
			has[lb] = true
		}
		if !has["public"] {
			r.add("manifest", Error, l.File, "lesson %s must carry the public label", l.ID)
		}
		if l.Payload != nil {
			if !has["encrypted"] || !has[l.Payload.Label] {
				r.add("manifest", Error, l.File, "lesson %s has a payload, so labels must include encrypted and %s", l.ID, l.Payload.Label)
			}
			if l.Challenge == nil {
				r.add("manifest", Warn, l.File, "lesson %s has a payload but no public challenge prompt", l.ID)
			}
		} else if has["encrypted"] || has["browser-decryptable"] {
			r.add("manifest", Error, l.File, "lesson %s is labelled encrypted but has no payload", l.ID)
		}
		if has["demo-only"] {
			r.add("manifest", Error, l.File, "lesson %s: demo-only belongs in demo/manifest.yaml, not the curriculum", l.ID)
		}
		if l.Status == "stable" && len(l.Vectors) == 0 {
			r.add("manifest", Error, l.File, "lesson %s is stable but has no verified vectors", l.ID)
		}
	}
	r.add("manifest", OK, "curriculum/", "%d lessons in %d manifests", len(lessons), len(paths))
	return lessons
}

func (r *run) checkVectorFiles(lessons []manifest.Lesson, opt Options) {
	files, err := vectors.LoadAll(r.root)
	if err != nil {
		r.add("vectors", Error, "vectors/", "%v", err)
		return
	}
	byPath := map[string]*vectors.File{}
	for _, f := range files {
		r.validateYAML("vectors", f.Path)
		byPath[f.Path] = f
	}
	referenced := map[string]bool{}
	for _, l := range lessons {
		for _, vp := range l.Vectors {
			referenced[vp] = true
			f, ok := byPath[vp]
			if !ok {
				r.add("vectors", Error, l.File, "lesson %s references missing vector file %s", l.ID, vp)
				continue
			}
			if f.LessonID != l.ID {
				r.add("vectors", Error, vp, "lesson_id %s does not match referencing lesson %s", f.LessonID, l.ID)
			}
			if l.Status == "stable" {
				for _, v := range f.Vectors {
					if v.Status != "verified" {
						r.add("vectors", Error, vp, "stable lesson %s includes unverified vector %s", l.ID, v.ID)
					}
				}
			}
		}
	}
	for _, f := range files {
		if !referenced[f.Path] && len(lessons) > 0 {
			r.add("vectors", Error, f.Path, "vector file is not referenced by any lesson")
		}
	}
	if opt.SkipVectors {
		r.add("vectors", Warn, "vectors/", "vector verification skipped")
		return
	}
	results := vectors.VerifyAll(files)
	pass := 0
	for _, res := range results {
		switch {
		case res.OK:
			pass++
			if res.Status == "unverified" {
				r.add("vectors", Warn, res.File, "%s passes but is labelled unverified", res.ID)
			}
		case res.Status == "verified":
			r.add("vectors", Error, res.File, "%s is labelled verified but fails: %s", res.ID, res.Error)
		default:
			r.add("vectors", Warn, res.File, "%s (unverified) fails: %s", res.ID, res.Error)
		}
	}
	r.add("vectors", OK, "vectors/", "%d/%d vectors pass", pass, len(results))
}

func (r *run) checkPayloads(lessons []manifest.Lesson) {
	referenced := map[string]bool{}
	for _, l := range lessons {
		if l.Payload == nil {
			continue
		}
		referenced[l.Payload.Path] = true
		r.checkPayloadFile(l.File, l.Payload.Path, l.Payload.SHA256)
	}
	if d, err := manifest.LoadDemo(r.root); err == nil {
		for _, p := range d.Payloads {
			referenced[p.Path] = true
		}
	}
	entries, err := os.ReadDir(filepath.Join(r.root, "payloads"))
	if err != nil {
		r.add("payloads", Error, "payloads/", "%v", err)
		return
	}
	n := 0
	for _, e := range entries {
		rel := "payloads/" + e.Name()
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".age") {
			r.add("payloads", Error, rel, "only .age files may live in payloads/")
			continue
		}
		n++
		if !referenced[rel] {
			r.add("payloads", Error, rel, "payload is not referenced by any manifest")
		}
	}
	r.add("payloads", OK, "payloads/", "%d encrypted payloads, all referenced", n)
}

func (r *run) checkPayloadFile(from, rel, digest string) []byte {
	b, err := os.ReadFile(filepath.Join(r.root, rel))
	if err != nil {
		r.add("payloads", Error, from, "payload %s: %v", rel, err)
		return nil
	}
	if !vault.IsAgeFile(b) {
		r.add("payloads", Error, rel, "not an age file (missing age header); plaintext must never be committed")
		return nil
	}
	if got := vault.SHA256Hex(b); got != digest {
		r.add("payloads", Error, rel, "sha256 %s does not match manifest digest %s", got, digest)
	}
	return b
}

func (r *run) checkDemo(lessons []manifest.Lesson) {
	rel := "demo/manifest.yaml"
	if !r.validateYAML("demo", rel) {
		return
	}
	d, err := manifest.LoadDemo(r.root)
	if err != nil {
		r.add("demo", Error, rel, "%v", err)
		return
	}
	ids, err := vault.LoadIdentities(filepath.Join(r.root, DemoIdentityPath))
	if err != nil || len(ids) != 1 {
		r.add("demo", Error, DemoIdentityPath, "demo identity must contain exactly one identity: %v", err)
		return
	}
	if rec := RecipientOf(ids[0]); rec != d.Recipient {
		r.add("demo", Error, rel, "demo recipient %s does not match the demo identity", d.Recipient)
	}
	for _, p := range d.Payloads {
		b := r.checkPayloadFile(rel, p.Path, p.SHA256)
		if b == nil {
			continue
		}
		pt, err := vault.Decrypt(b, ids)
		if err != nil {
			r.add("demo", Error, p.Path, "demo identity cannot decrypt demo payload: %v", err)
			continue
		}
		if got := vault.SHA256Hex(pt); got != p.PlaintextSHA256 {
			r.add("demo", Error, p.Path, "demo plaintext sha256 %s does not match %s", got, p.PlaintextSHA256)
		}
	}
	// The demo identity must not open any real payload.
	for _, l := range lessons {
		if l.Payload == nil {
			continue
		}
		b, err := os.ReadFile(filepath.Join(r.root, l.Payload.Path))
		if err != nil {
			continue
		}
		if _, err := vault.Decrypt(b, ids); err == nil {
			r.add("demo", Error, l.Payload.Path, "real payload is decryptable with the public demo identity")
		}
	}
	if entries, err := vault.LoadRecipients(filepath.Join(r.root, "recipients", "maintainers.txt")); err == nil {
		for _, e := range entries {
			if e.Value == d.Recipient {
				r.add("demo", Error, "recipients/maintainers.txt", "the demo recipient must not be an enrolled maintainer recipient")
			}
		}
	}
	r.add("demo", OK, rel, "%d demo payloads open with the demo identity; no real payload does", len(d.Payloads))
}

// RecipientOf returns the public recipient string of a native identity.
func RecipientOf(id age.Identity) string {
	switch v := id.(type) {
	case *age.X25519Identity:
		return v.Recipient().String()
	case *age.HybridIdentity:
		return v.Recipient().String()
	}
	return ""
}

func (r *run) checkRecipients() {
	rel := "recipients/maintainers.txt"
	entries, err := vault.LoadRecipients(filepath.Join(r.root, rel))
	if err != nil {
		r.add("recipients", Error, rel, "%v", err)
		return
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.Value] {
			r.add("recipients", Error, rel, "line %d: duplicate recipient", e.Line)
		}
		seen[e.Value] = true
		if strings.HasPrefix(e.Kind, "ssh-") {
			r.add("recipients", Warn, rel, "line %d: SSH recipient; prefer a native age key", e.Line)
		}
	}
	r.add("recipients", OK, rel, "%d enrolled recipients", len(entries))
}

type sopsConfig struct {
	CreationRules []struct {
		PathRegex      string `yaml:"path_regex"`
		EncryptedRegex string `yaml:"encrypted_regex"`
		Age            string `yaml:"age"`
	} `yaml:"creation_rules"`
}

func (r *run) checkSOPS() {
	b, err := os.ReadFile(filepath.Join(r.root, ".sops.yaml"))
	if err != nil {
		r.add("sops", Error, ".sops.yaml", "%v", err)
		return
	}
	var cfg sopsConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil || len(cfg.CreationRules) == 0 {
		r.add("sops", Error, ".sops.yaml", "no creation_rules: %v", err)
		return
	}
	matches, _ := filepath.Glob(filepath.Join(r.root, "private-cli", "*"))
	n := 0
	for _, m := range matches {
		rel, _ := filepath.Rel(r.root, m)
		rel = filepath.ToSlash(rel)
		if !strings.HasSuffix(rel, ".sops.yaml") {
			r.add("sops", Error, rel, "only *.sops.yaml files may live in private-cli/")
			continue
		}
		var rule *regexp.Regexp
		for _, cr := range cfg.CreationRules {
			if ok, _ := regexp.MatchString(cr.PathRegex, rel); ok {
				rule = regexp.MustCompile(cr.EncryptedRegex)
				break
			}
		}
		if rule == nil {
			r.add("sops", Error, rel, "no .sops.yaml creation rule matches this file")
			continue
		}
		var doc map[string]any
		fb, _ := os.ReadFile(m)
		if err := yaml.Unmarshal(fb, &doc); err != nil {
			r.add("sops", Error, rel, "invalid YAML: %v", err)
			continue
		}
		meta, ok := doc["sops"].(map[string]any)
		if !ok || meta["mac"] == nil {
			r.add("sops", Error, rel, "missing SOPS metadata; the file is not encrypted")
			continue
		}
		if bad := plaintextSOPSValues(doc, rule, ""); len(bad) > 0 {
			r.add("sops", Error, rel, "values not encrypted for keys: %s", strings.Join(bad, ", "))
			continue
		}
		n++
	}
	r.add("sops", OK, "private-cli/", "%d SOPS files with encrypted selected values", n)
}

func plaintextSOPSValues(v any, rule *regexp.Regexp, prefix string) []string {
	var bad []string
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if k == "sops" && prefix == "" {
			continue
		}
		val := m[k]
		if rule.MatchString(k) {
			if s, ok := val.(string); !ok || !strings.HasPrefix(s, "ENC[AES256_GCM,") {
				bad = append(bad, prefix+k)
			}
			continue
		}
		bad = append(bad, plaintextSOPSValues(val, rule, prefix+k+".")...)
	}
	return bad
}

var skipDirs = map[string]bool{".git": true, "node_modules": true}

func (r *run) checkPrivateMaterial(extra []string) {
	// The demo identity is allowed only at exactly demo/demo-identity.txt
	// relative to the scanned directory (the repository or a built site).
	// Built sites passed with --site are scanned as their own base, so the
	// repository scan skips them (they may live inside the checkout).
	separate := map[string]bool{}
	for _, d := range extra {
		if abs, err := filepath.Abs(d); err == nil {
			separate[abs] = true
		}
	}
	scan := func(base string, isRepo bool) int {
		hits := 0
		if abs, err := filepath.Abs(base); err == nil {
			base = abs
		}
		filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if skipDirs[d.Name()] || (isRepo && (d.Name() == "_site" || (p != base && separate[p]))) {
					return filepath.SkipDir
				}
				return nil
			}
			relBase, err := filepath.Rel(base, p)
			if err != nil {
				relBase = p
			}
			relBase = filepath.ToSlash(relBase)
			rel := relBase
			if !isRepo {
				rel = filepath.ToSlash(p)
			}
			if strings.HasSuffix(p, ".age") {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil || len(b) > 32<<20 {
				return nil
			}
			allowed := relBase == DemoIdentityPath
			for _, pat := range vault.PrivatePatterns {
				if pat.Match(b) && !allowed {
					r.add("secrets", Error, rel, "contains private key material matching %s", pat.String())
					hits++
					break
				}
			}
			return nil
		})
		return hits
	}
	hits := scan(r.root, true)
	for _, d := range extra {
		hits += scan(d, false)
	}
	if hits == 0 {
		r.add("secrets", OK, "", "no private identities outside %s", DemoIdentityPath)
	}
}

var forbidden = []*regexp.Regexp{
	regexp.MustCompile(`(^|/)plaintext-source/`),
	regexp.MustCompile(`(^|/)private-work/`),
	regexp.MustCompile(`\.decrypted\.`),
	regexp.MustCompile(`\.plaintext\.`),
	regexp.MustCompile(`(^|/)keys\.txt$`),
	regexp.MustCompile(`\.(agekey|private-key|identity)$`),
	regexp.MustCompile(`(^|/)\.env(\.|$)`),
}

func (r *run) checkTrackedPaths() {
	files := trackedFiles(r.root)
	hits := 0
	for _, f := range files {
		if f == ".env.example" {
			continue
		}
		for _, re := range forbidden {
			if re.MatchString(f) {
				r.add("tracked", Error, f, "plaintext or key file pattern %s must not be in the repository", re.String())
				hits++
				break
			}
		}
	}
	if hits == 0 {
		r.add("tracked", OK, "", "%d files checked; no plaintext sources or key files", len(files))
	}
}

func trackedFiles(root string) []string {
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		cmd := exec.Command("git", "-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
		if out, err := cmd.Output(); err == nil {
			var files []string
			for _, f := range strings.Split(string(out), "\x00") {
				if f != "" {
					files = append(files, f)
				}
			}
			return files
		}
	}
	var files []string
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (skipDirs[d.Name()] || d.Name() == "_site") {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(root, p)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	return files
}

var remoteRef = regexp.MustCompile(`(?i)<(script|link|img|iframe)[^>]+(src|href)\s*=\s*["']?(https?:)?//`)

func (r *run) checkPortal() {
	idx := "docs/index.html"
	b, err := os.ReadFile(filepath.Join(r.root, idx))
	if err != nil {
		r.add("portal", Error, idx, "%v", err)
		return
	}
	s := string(b)
	if !strings.Contains(s, `http-equiv="Content-Security-Policy"`) {
		r.add("portal", Error, idx, "missing Content-Security-Policy meta element")
	}
	if !strings.Contains(s, "connect-src 'self'") {
		r.add("portal", Error, idx, "CSP must use connect-src 'self' (profile B: separate same-origin payload files)")
	}
	if strings.Contains(s, "'unsafe-inline'") || strings.Contains(s, "'unsafe-eval'") {
		r.add("portal", Error, idx, "CSP must not allow unsafe-inline or unsafe-eval")
	}
	if remoteRef.MatchString(s) {
		r.add("portal", Error, idx, "portal loads a remote resource; bundle every dependency locally")
	}
	sc := bufio.NewScanner(strings.NewReader(s))
	line := 0
	for sc.Scan() {
		line++
		t := sc.Text()
		if strings.Contains(t, "<script>") || strings.Contains(t, "<style") || strings.Contains(t, " style=\"") || strings.Contains(t, " on") && regexp.MustCompile(`\son[a-z]+\s*=`).MatchString(t) {
			r.add("portal", Error, idx, "line %d: inline script, style or event handler is not allowed by the CSP", line)
		}
	}
	for _, f := range []string{"docs/app.js", "docs/styles.css", "docs/assets/age.js"} {
		if _, err := os.Stat(filepath.Join(r.root, f)); err != nil {
			r.add("portal", Error, f, "missing portal file")
		}
	}
	r.add("portal", OK, "docs/", "static portal with self-only CSP")
}

// Join is a tiny helper for callers building repo-relative paths.
func Join(parts ...string) string { return path.Join(parts...) }
