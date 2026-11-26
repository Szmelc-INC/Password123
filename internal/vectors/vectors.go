// Package vectors loads public test vector files and verifies every vector by
// recomputing it with a maintained implementation (Go standard library,
// golang.org/x/crypto, filippo.io/age, or this repository's teaching code).
package vectors

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Szmelc-INC/Password123/internal/enc"
	"gopkg.in/yaml.v3"
)

// Kinds of vectors. Only "official", "reference-implementation" and
// "community" entries are published test vectors. "adapted" entries are
// teaching examples, "parameter" entries check published constants, and
// "property" entries are self-checks, not vectors.
var Kinds = []string{"official", "reference-implementation", "community", "adapted", "parameter", "property"}

// Source names where a vector comes from.
type Source struct {
	Document string `yaml:"document" json:"document"`
	Section  string `yaml:"section,omitempty" json:"section,omitempty"`
	TestCase string `yaml:"test_case,omitempty" json:"test_case,omitempty"`
	URL      string `yaml:"url" json:"url"`
}

// Vector is one test vector or check.
type Vector struct {
	ID       string               `yaml:"id" json:"id"`
	Kind     string               `yaml:"kind" json:"kind"`
	Status   string               `yaml:"status" json:"status"`
	Verifier string               `yaml:"verifier" json:"verifier"`
	Source   Source               `yaml:"source" json:"source"`
	Params   map[string]enc.Value `yaml:"params,omitempty" json:"params,omitempty"`
	Inputs   map[string]enc.Value `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	Expected map[string]enc.Value `yaml:"expected,omitempty" json:"expected,omitempty"`
	Notes    string               `yaml:"notes,omitempty" json:"notes,omitempty"`
}

// File is one vectors/**.yaml document.
type File struct {
	SchemaVersion int      `yaml:"schema_version" json:"schema_version"`
	LessonID      string   `yaml:"lesson_id" json:"lesson_id"`
	Algorithm     string   `yaml:"algorithm" json:"algorithm"`
	Vectors       []Vector `yaml:"vectors" json:"vectors"`
	Path          string   `yaml:"-" json:"path"`
}

// Result is the outcome of verifying one vector.
type Result struct {
	File     string `json:"file"`
	LessonID string `json:"lesson_id"`
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Status   string `json:"status"`
	Verifier string `json:"verifier"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
}

// Load reads one vector file.
func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f File
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	f.Path = path
	return &f, nil
}

// LoadAll reads every vectors/**.yaml file under root, sorted by path.
func LoadAll(root string) ([]*File, error) {
	var paths []string
	err := filepath.WalkDir(filepath.Join(root, "vectors"), func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".yaml") {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var out []*File
	for _, p := range paths {
		f, err := Load(p)
		if err != nil {
			return nil, err
		}
		rel, _ := filepath.Rel(root, p)
		f.Path = filepath.ToSlash(rel)
		out = append(out, f)
	}
	return out, nil
}

// Verify runs one vector through its registered verifier.
func Verify(v Vector) error {
	fn, ok := registry[v.Verifier]
	if !ok {
		return fmt.Errorf("unknown verifier %q", v.Verifier)
	}
	for name, val := range v.Params {
		if _, err := val.Encoding(); err != nil {
			return fmt.Errorf("params.%s: %w", name, err)
		}
	}
	for name, val := range v.Inputs {
		if _, err := val.Encoding(); err != nil {
			return fmt.Errorf("inputs.%s: %w", name, err)
		}
	}
	for name, val := range v.Expected {
		if _, err := val.Encoding(); err != nil {
			return fmt.Errorf("expected.%s: %w", name, err)
		}
	}
	return fn(&args{v: v})
}

// VerifyAll verifies every vector in files. A vector with status "verified"
// that fails is an error; callers decide how to treat "unverified" passes.
func VerifyAll(files []*File) []Result {
	var out []Result
	for _, f := range files {
		for _, v := range f.Vectors {
			r := Result{File: f.Path, LessonID: f.LessonID, ID: v.ID, Kind: v.Kind, Status: v.Status, Verifier: v.Verifier}
			if err := Verify(v); err != nil {
				r.Error = err.Error()
			} else {
				r.OK = true
			}
			out = append(out, r)
		}
	}
	return out
}

// Verifiers lists registered verifier names.
func Verifiers() []string {
	var names []string
	for k := range registry {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
