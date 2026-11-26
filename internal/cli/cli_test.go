package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root, _ := filepath.Abs("../..")
	cmd := NewRoot()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"--repo", root, "--config", filepath.Join(t.TempDir(), "none.yaml")}, args...))
	err := cmd.Execute()
	return out.String(), err
}

func TestHelpHasGroups(t *testing.T) {
	out, err := run(t, "--help", "--no-color")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Learn", "Vault", "Repository", "encrypt", "completion", "Global flags"} {
		if !strings.Contains(out, want) {
			t.Errorf("help lacks %q", want)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("--no-color output contains ANSI escapes")
	}
}

func TestCompletionScripts(t *testing.T) {
	for _, sh := range []string{"bash", "zsh", "fish"} {
		out, err := run(t, "completion", sh)
		if err != nil || !strings.Contains(out, "p123") {
			t.Fatalf("%s completion: %v", sh, err)
		}
	}
}

func TestUnknownLessonFails(t *testing.T) {
	if _, err := run(t, "lessons", "show", "99-nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestJSONIsValid(t *testing.T) {
	out, err := run(t, "lessons", "list", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var lessons []map[string]any
	if err := json.Unmarshal([]byte(out), &lessons); err != nil || len(lessons) < 20 {
		t.Fatalf("invalid JSON or too few lessons: %v (%d)", err, len(lessons))
	}
}
