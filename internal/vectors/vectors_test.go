package vectors

import (
	"path/filepath"
	"testing"

	"github.com/Szmelc-INC/Password123/internal/enc"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// TestRepositoryVectors verifies every vector file shipped in the repository.
// A vector labelled "verified" must pass; that is what earns it the label.
func TestRepositoryVectors(t *testing.T) {
	files, err := LoadAll(repoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no vector files found")
	}
	for _, f := range files {
		for _, v := range f.Vectors {
			t.Run(f.LessonID+"/"+v.ID, func(t *testing.T) {
				if testing.Short() && v.ID == "cctv-accumulated-10000" {
					t.Skip("long accumulated vector")
				}
				err := Verify(v)
				if v.Status == "verified" && err != nil {
					t.Fatalf("verified vector fails: %v", err)
				}
			})
		}
	}
}

// TestWrongExpectedValueFails makes sure verifiers compare, not just compute.
func TestWrongExpectedValueFails(t *testing.T) {
	root := repoRoot(t)
	for _, tc := range []struct{ file, field string }{
		{"vectors/symmetric/aes-256-gcm.yaml", "tag"},
		{"vectors/symmetric/chacha20-poly1305.yaml", "ciphertext"},
		{"vectors/asymmetric/ed25519.yaml", "signature"},
		{"vectors/hashes-kdfs/sha-256.yaml", "digest"},
		{"vectors/post-quantum/ml-kem-768.yaml", "accumulated"},
	} {
		f, err := Load(filepath.Join(root, tc.file))
		if err != nil {
			t.Fatal(err)
		}
		v := f.Vectors[0]
		good, err := v.Expected[tc.field].Bytes()
		if err != nil {
			t.Fatal(err)
		}
		bad := append([]byte(nil), good...)
		bad[0] ^= 0xff
		exp := map[string]enc.Value{}
		for k, val := range v.Expected {
			exp[k] = val
		}
		exp[tc.field] = enc.Hex(bad)
		v.Expected = exp
		if err := Verify(v); err == nil {
			t.Errorf("%s: wrong expected %s was accepted", tc.file, tc.field)
		}
	}
}

func TestUnknownVerifier(t *testing.T) {
	if err := Verify(Vector{Verifier: "nope"}); err == nil {
		t.Fatal("unknown verifier must fail")
	}
}
