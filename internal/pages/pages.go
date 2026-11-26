// Package pages builds the static GitHub Pages portal from public manifests
// and encrypted payloads only. The build is deterministic: the same commit
// produces byte-identical output, so a third party can rebuild and compare.
package pages

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Szmelc-INC/Password123/internal/manifest"
	"github.com/Szmelc-INC/Password123/internal/vault"
	"github.com/Szmelc-INC/Password123/internal/vectors"
)

// CSP is the policy sent by `p123 pages serve`; docs/index.html carries the same policy in a meta element.
const CSP = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

// SiteManifest is generated/manifest.json.
type SiteManifest struct {
	SchemaVersion int               `json:"schema_version"`
	Source        BuildInfo         `json:"source"`
	Lessons       []SiteLesson      `json:"lessons"`
	Demo          *manifest.Demo    `json:"demo"`
	Vectors       []vectors.Result  `json:"vector_results"`
	Modules       []string          `json:"modules"`
	VectorFiles   []*vectors.File   `json:"vector_files"`
	Recipients    []string          `json:"recipients"`
	Extra         map[string]string `json:"extra,omitempty"`
}

// SiteLesson is a lesson plus derived flags used by the portal.
type SiteLesson struct {
	manifest.Lesson
	VectorCount int `json:"vector_count"`
}

// BuildInfo records what the site was built from.
type BuildInfo struct {
	Revision string `json:"revision"`
	Builder  string `json:"builder"`
}

// Options for Build.
type Options struct {
	OutDir  string
	Version string
}

// Build writes the site into opt.OutDir, replacing previous contents.
func Build(root string, opt Options) ([]string, error) {
	out := opt.OutDir
	if out == "" {
		out = filepath.Join(root, "_site")
	}
	if vault.Inside(out, root) {
		return nil, fmt.Errorf("output directory %s contains the repository", out)
	}
	if err := os.RemoveAll(out); err != nil {
		return nil, err
	}
	lessons, err := manifest.LoadLessons(root)
	if err != nil {
		return nil, err
	}
	demo, err := manifest.LoadDemo(root)
	if err != nil {
		return nil, err
	}
	vfiles, err := vectors.LoadAll(root)
	if err != nil {
		return nil, err
	}
	results := vectors.VerifyAll(vfiles)
	for _, r := range results {
		if !r.OK && r.Status == "verified" {
			return nil, fmt.Errorf("refusing to build: verified vector %s/%s fails: %s", r.LessonID, r.ID, r.Error)
		}
	}
	// 1. static portal files from docs/
	if err := copyTree(filepath.Join(root, "docs"), out); err != nil {
		return nil, err
	}
	// 2. encrypted payloads referenced by manifests
	var payloads []string
	for _, l := range lessons {
		if l.Payload != nil {
			payloads = append(payloads, l.Payload.Path)
		}
	}
	for _, p := range demo.Payloads {
		payloads = append(payloads, p.Path)
	}
	for _, p := range payloads {
		b, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			return nil, err
		}
		if !vault.IsAgeFile(b) {
			return nil, fmt.Errorf("refusing to publish %s: not an age file", p)
		}
		if err := writeFile(filepath.Join(out, p), b); err != nil {
			return nil, err
		}
	}
	// 3. the demo identity (public by design, demo payloads only)
	if b, err := os.ReadFile(filepath.Join(root, demo.Identity)); err == nil {
		if err := writeFile(filepath.Join(out, demo.Identity), b); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	// 4. public recipients so visitors can see who can decrypt what
	var recips []string
	if entries, err := vault.LoadRecipients(filepath.Join(root, "recipients", "maintainers.txt")); err == nil {
		for _, e := range entries {
			recips = append(recips, e.Value)
		}
	}
	// 5. generated manifest
	site := SiteManifest{
		SchemaVersion: 1,
		Source:        BuildInfo{Revision: gitRevision(root), Builder: "p123 " + opt.Version},
		Demo:          demo,
		Vectors:       results,
		VectorFiles:   vfiles,
		Recipients:    recips,
	}
	mods := map[string]bool{}
	for _, l := range lessons {
		n := 0
		for _, vp := range l.Vectors {
			for _, f := range vfiles {
				if f.Path == vp {
					n += len(f.Vectors)
				}
			}
		}
		site.Lessons = append(site.Lessons, SiteLesson{Lesson: l, VectorCount: n})
		mods[l.Module] = true
	}
	for m := range mods {
		site.Modules = append(site.Modules, m)
	}
	sort.Strings(site.Modules)
	mj, err := json.MarshalIndent(site, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := writeFile(filepath.Join(out, "generated", "manifest.json"), append(mj, '\n')); err != nil {
		return nil, err
	}
	// 6. checksums over every published file
	files, sums, err := checksums(out)
	if err != nil {
		return nil, err
	}
	if err := writeFile(filepath.Join(out, "SHA256SUMS"), []byte(sums)); err != nil {
		return nil, err
	}
	return files, nil
}

func writeFile(p string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o644)
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			if d.Name() == "generated" || d.Name() == "payloads" {
				return filepath.SkipDir
			}
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return writeFile(filepath.Join(dst, rel), b)
	})
}

func checksums(dir string) ([]string, string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			if rel != "SHA256SUMS" {
				files = append(files, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	sort.Strings(files)
	var b strings.Builder
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			return nil, "", err
		}
		fmt.Fprintf(&b, "%s  %s\n", vault.SHA256Hex(data), f)
	}
	return files, b.String(), nil
}

func gitRevision(root string) string {
	if v := os.Getenv("GITHUB_SHA"); v != "" {
		return v
	}
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// Handler serves a built site with the portal CSP and safe headers.
func Handler(dir string) http.Handler {
	fsrv := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", CSP)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		if strings.HasSuffix(r.URL.Path, ".age") {
			h.Set("Content-Type", "application/octet-stream")
		}
		fsrv.ServeHTTP(w, r)
	})
}
