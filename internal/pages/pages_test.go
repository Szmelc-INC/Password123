package pages

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildIsDeterministic(t *testing.T) {
	root, _ := filepath.Abs("../..")
	a, b := filepath.Join(t.TempDir(), "a"), filepath.Join(t.TempDir(), "b")
	for _, d := range []string{a, b} {
		if _, err := Build(root, Options{OutDir: d, Version: "test"}); err != nil {
			t.Fatal(err)
		}
	}
	sa, _ := os.ReadFile(filepath.Join(a, "SHA256SUMS"))
	sb, _ := os.ReadFile(filepath.Join(b, "SHA256SUMS"))
	if len(sa) == 0 || string(sa) != string(sb) {
		t.Fatal("two builds of the same tree differ")
	}
	if _, err := os.Stat(filepath.Join(a, "generated", "manifest.json")); err != nil {
		t.Fatal(err)
	}
}
