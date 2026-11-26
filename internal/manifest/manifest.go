// Package manifest loads the public lesson manifests under curriculum/ and the
// demo manifest under demo/. Manifests are the public index: everything in
// them is intentionally readable by everyone.
package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Allowed values. Keep these in sync with schemas/lesson.schema.json.
var (
	Statuses     = []string{"draft", "experimental", "stable", "deprecated"}
	Difficulties = []string{"beginner", "intermediate", "advanced"}
	Labels       = []string{"public", "encrypted", "private-cli", "browser-decryptable", "demo-only"}
)

// Reference is a public citation.
type Reference struct {
	Name string `yaml:"name" json:"name"`
	URL  string `yaml:"url" json:"url"`
}

// Guide is the lesson text, following the documentation style in CONTRIBUTING.md.
type Guide struct {
	What           string   `yaml:"what" json:"what"`
	Problem        string   `yaml:"problem" json:"problem"`
	InputsOutputs  string   `yaml:"inputs_outputs" json:"inputs_outputs"`
	KeyManagement  string   `yaml:"key_management" json:"key_management"`
	CommonMistakes []string `yaml:"common_mistakes" json:"common_mistakes"`
	NewSystems     string   `yaml:"new_systems" json:"new_systems"`
	Reproduce      string   `yaml:"reproduce" json:"reproduce"`
}

// Challenge is the public part of a challenge. Its answer lives in the payload.
type Challenge struct {
	Prompt       string         `yaml:"prompt" json:"prompt"`
	PublicInputs map[string]any `yaml:"public_inputs" json:"public_inputs"`
	Submit       string         `yaml:"submit,omitempty" json:"submit,omitempty"`
}

// Payload points to an encrypted file.
type Payload struct {
	Path   string `yaml:"path" json:"path"`
	Format string `yaml:"format" json:"format"`
	SHA256 string `yaml:"sha256" json:"sha256"`
	Label  string `yaml:"label" json:"label"`
}

// Lesson is one public lesson entry.
type Lesson struct {
	ID                string      `yaml:"id" json:"id"`
	Title             string      `yaml:"title" json:"title"`
	Module            string      `yaml:"module" json:"module"`
	Category          string      `yaml:"category" json:"category"`
	Difficulty        string      `yaml:"difficulty" json:"difficulty"`
	Status            string      `yaml:"status" json:"status"`
	Summary           string      `yaml:"summary" json:"summary"`
	RecommendedForNew bool        `yaml:"recommended_for_new_systems" json:"recommended_for_new_systems"`
	Labels            []string    `yaml:"labels" json:"labels"`
	References        []Reference `yaml:"references" json:"references"`
	Vectors           []string    `yaml:"vectors" json:"vectors"`
	Guide             Guide       `yaml:"guide" json:"guide"`
	Challenge         *Challenge  `yaml:"challenge,omitempty" json:"challenge,omitempty"`
	Payload           *Payload    `yaml:"payload,omitempty" json:"payload,omitempty"`

	File string `yaml:"-" json:"file"`
}

// File is one curriculum/**.yaml document.
type File struct {
	SchemaVersion int      `yaml:"schema_version"`
	Lessons       []Lesson `yaml:"lessons"`
}

// DemoPayload is a demo-only payload, decryptable with the published demo identity.
type DemoPayload struct {
	ID              string `yaml:"id" json:"id"`
	Title           string `yaml:"title" json:"title"`
	Path            string `yaml:"path" json:"path"`
	Format          string `yaml:"format" json:"format"`
	SHA256          string `yaml:"sha256" json:"sha256"`
	PlaintextSHA256 string `yaml:"plaintext_sha256" json:"plaintext_sha256"`
	Label           string `yaml:"label" json:"label"`
}

// Demo is demo/manifest.yaml.
type Demo struct {
	SchemaVersion int           `yaml:"schema_version" json:"schema_version"`
	Identity      string        `yaml:"identity" json:"identity"`
	Recipient     string        `yaml:"recipient" json:"recipient"`
	Payloads      []DemoPayload `yaml:"payloads" json:"payloads"`
}

func decodeStrict(path string, out any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// ManifestPaths lists every curriculum/**.yaml file, repository-relative and sorted.
func ManifestPaths(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(filepath.Join(root, "curriculum"), func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".yaml") {
			rel, _ := filepath.Rel(root, p)
			paths = append(paths, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(paths)
	return paths, err
}

// LoadLessons loads every lesson, sorted by ID.
func LoadLessons(root string) ([]Lesson, error) {
	paths, err := ManifestPaths(root)
	if err != nil {
		return nil, err
	}
	var out []Lesson
	for _, rel := range paths {
		var f File
		if err := decodeStrict(filepath.Join(root, rel), &f); err != nil {
			return nil, err
		}
		if f.SchemaVersion != 1 {
			return nil, fmt.Errorf("%s: unsupported schema_version %d", rel, f.SchemaVersion)
		}
		for _, l := range f.Lessons {
			l.File = rel
			out = append(out, l)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// LoadDemo loads demo/manifest.yaml.
func LoadDemo(root string) (*Demo, error) {
	var d Demo
	if err := decodeStrict(filepath.Join(root, "demo", "manifest.yaml"), &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// Find returns the lesson with id.
func Find(lessons []Lesson, id string) (*Lesson, bool) {
	for i := range lessons {
		if lessons[i].ID == id {
			return &lessons[i], true
		}
	}
	return nil, false
}

// SetPayloadDigest rewrites the payload sha256 of lesson id in its manifest
// file. It edits the YAML node tree so comments and ordering survive.
func SetPayloadDigest(root string, l *Lesson, digest string) error {
	path := filepath.Join(root, l.File)
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return err
	}
	if !setDigest(&doc, l.ID, digest) {
		return fmt.Errorf("%s: no payload.sha256 found for lesson %s", l.File, l.ID)
	}
	out, err := marshal2(&doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

// SetDemoDigest rewrites the sha256 of a demo payload.
func SetDemoDigest(root, id, digest, plaintextDigest string) error {
	path := filepath.Join(root, "demo", "manifest.yaml")
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return err
	}
	found := false
	walk(&doc, func(n *yaml.Node) {
		if n.Kind != yaml.MappingNode || mapValue(n, "id") == nil || mapValue(n, "id").Value != id {
			return
		}
		if v := mapValue(n, "sha256"); v != nil {
			v.Value, found = digest, true
		}
		if v := mapValue(n, "plaintext_sha256"); v != nil && plaintextDigest != "" {
			v.Value = plaintextDigest
		}
	})
	if !found {
		return fmt.Errorf("demo payload %s not found", id)
	}
	out, err := marshal2(&doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func marshal2(n *yaml.Node) ([]byte, error) {
	var b strings.Builder
	e := yaml.NewEncoder(&b)
	e.SetIndent(2)
	if err := e.Encode(n); err != nil {
		return nil, err
	}
	if err := e.Close(); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

func mapValue(n *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func walk(n *yaml.Node, fn func(*yaml.Node)) {
	fn(n)
	for _, c := range n.Content {
		walk(c, fn)
	}
}

func setDigest(doc *yaml.Node, id, digest string) bool {
	found := false
	walk(doc, func(n *yaml.Node) {
		if n.Kind != yaml.MappingNode {
			return
		}
		idNode := mapValue(n, "id")
		if idNode == nil || idNode.Value != id {
			return
		}
		if p := mapValue(n, "payload"); p != nil {
			if s := mapValue(p, "sha256"); s != nil {
				s.Value = digest
				found = true
			}
		}
	})
	return found
}
