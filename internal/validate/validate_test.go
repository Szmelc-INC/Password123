package validate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// copyRepo copies the working tree (without .git and local-only dirs) to a temp dir.
func copyRepo(t *testing.T) string {
	t.Helper()
	src, _ := filepath.Abs("../..")
	dst := t.TempDir()
	if out, err := exec.Command("cp", "-a", src+"/.", dst).CombinedOutput(); err != nil {
		t.Fatalf("copy: %v %s", err, out)
	}
	for _, d := range []string{".git", "web/node_modules", "private-work", "_site"} {
		os.RemoveAll(filepath.Join(dst, d))
	}
	return dst
}

func errorsFor(fs []Finding, check string) []Finding {
	var out []Finding
	for _, f := range fs {
		if f.Level == Error && (check == "" || f.Check == check) {
			out = append(out, f)
		}
	}
	return out
}

func TestRepositoryPasses(t *testing.T) {
	root, _ := filepath.Abs("../..")
	if errs := errorsFor(Run(root, Options{SkipVectors: true}), ""); len(errs) > 0 {
		t.Fatalf("repository fails validation: %+v", errs)
	}
}

func replaceIn(t *testing.T, p, old, new string) {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), old) {
		t.Fatalf("%s does not contain %q", p, old)
	}
	os.WriteFile(p, []byte(strings.Replace(string(b), old, new, 1)), 0o644)
}

// TestFailClosed breaks the repository in one way per case and expects an error.
func TestFailClosed(t *testing.T) {
	cases := []struct {
		name, check string
		breakIt     func(t *testing.T, root string)
	}{
		{"plaintext payload", "payloads", func(t *testing.T, root string) {
			os.WriteFile(filepath.Join(root, "payloads/01-caesar.age"), []byte(`{"solution":"leak"}`), 0o644)
		}},
		{"digest mismatch", "payloads", func(t *testing.T, root string) {
			p := filepath.Join(root, "payloads/02-rc4.age")
			b, _ := os.ReadFile(p)
			os.WriteFile(p, append(b, 0), 0o644)
		}},
		{"orphan payload", "payloads", func(t *testing.T, root string) {
			b, _ := os.ReadFile(filepath.Join(root, "payloads/02-rc4.age"))
			os.WriteFile(filepath.Join(root, "payloads/zz-orphan.age"), b, 0o644)
		}},
		{"private identity committed", "secrets", func(t *testing.T, root string) {
			os.WriteFile(filepath.Join(root, "notes.txt"), []byte("AGE-SECRET-KEY-"+"1"+strings.Repeat("QYQSZQGP", 8)), 0o644)
		}},
		{"plaintext source directory", "tracked", func(t *testing.T, root string) {
			os.MkdirAll(filepath.Join(root, "plaintext-source"), 0o755)
			os.WriteFile(filepath.Join(root, "plaintext-source/answer.json"), []byte("{}"), 0o644)
		}},
		{"duplicate lesson id", "manifest", func(t *testing.T, root string) {
			b, _ := os.ReadFile(filepath.Join(root, "curriculum/01-classical/caesar.yaml"))
			os.WriteFile(filepath.Join(root, "curriculum/01-classical/caesar-copy.yaml"), b, 0o644)
		}},
		{"unknown status", "schema", func(t *testing.T, root string) {
			replaceIn(t, filepath.Join(root, "curriculum/02-stream-and-block/rc4.yaml"), `status: "deprecated"`, `status: "shipped"`)
		}},
		{"missing vector file", "vectors", func(t *testing.T, root string) {
			os.Remove(filepath.Join(root, "vectors/hashes-kdfs/hmac.yaml"))
		}},
		{"unencrypted sops value", "sops", func(t *testing.T, root string) {
			p := filepath.Join(root, "private-cli/maintainer.sops.yaml")
			b, _ := os.ReadFile(p)
			lines := strings.Split(string(b), "\n")
			for i, l := range lines {
				if strings.HasPrefix(l, "answer:") {
					lines[i] = "answer: plaintext-answer"
				}
			}
			os.WriteFile(p, []byte(strings.Join(lines, "\n")), 0o644)
		}},
		{"remote script in portal", "portal", func(t *testing.T, root string) {
			replaceIn(t, filepath.Join(root, "docs/index.html"), "</head>", `<script src="https://cdn.example/x.js"></script></head>`)
		}},
		{"demo identity opens a real payload", "demo", func(t *testing.T, root string) {
			b, _ := os.ReadFile(filepath.Join(root, "payloads/demo-welcome.age"))
			os.WriteFile(filepath.Join(root, "payloads/01-caesar.age"), b, 0o644)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := copyRepo(t)
			tc.breakIt(t, root)
			if len(errorsFor(Run(root, Options{SkipVectors: true}), tc.check)) == 0 {
				t.Fatalf("expected a %s error", tc.check)
			}
		})
	}
}
