package vault

import (
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
)

func TestRoundTripAndWrongIdentity(t *testing.T) {
	for _, armored := range []bool{false, true} {
		id, _ := age.GenerateX25519Identity()
		pq, _ := age.GenerateHybridIdentity()
		other, _ := age.GenerateX25519Identity()
		if _, err := Encrypt([]byte("x"), []age.Recipient{id.Recipient(), pq.Recipient()}, armored); err == nil {
			t.Fatal("mixing classic and post-quantum recipients must be refused")
		}
		pairs := []struct {
			r age.Recipient
			i age.Identity
		}{{id.Recipient(), id}, {pq.Recipient(), pq}}
		for _, p := range pairs {
			ct, err := Encrypt([]byte("secret"), []age.Recipient{p.r}, armored)
			if err != nil {
				t.Fatal(err)
			}
			if !IsAgeFile(ct) {
				t.Fatal("missing age header")
			}
			pt, err := Decrypt(ct, []age.Identity{p.i})
			if err != nil || string(pt) != "secret" {
				t.Fatalf("decrypt: %v %q", err, pt)
			}
			if _, err := Decrypt(ct, []age.Identity{other}); err == nil {
				t.Fatal("unrelated identity decrypted")
			}
		}
	}
}

func TestInsideAndWriteSecret(t *testing.T) {
	repo := t.TempDir()
	if !Inside(repo, filepath.Join(repo, "a", "b.json")) || Inside(repo, filepath.Join(repo, "..", "x")) {
		t.Fatal("Inside is wrong")
	}
	if err := WriteSecret(repo, filepath.Join(repo, "leak.json"), []byte("x")); err != ErrPlaintextInRepo {
		t.Fatalf("expected refusal, got %v", err)
	}
	out := filepath.Join(t.TempDir(), "ok.json")
	if err := WriteSecret(repo, out, []byte("x")); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(out); st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", st.Mode().Perm())
	}
}

func TestRecipientsRejectPrivateKeys(t *testing.T) {
	id, _ := age.GenerateX25519Identity()
	p := filepath.Join(t.TempDir(), "r.txt")
	os.WriteFile(p, []byte(id.String()+"\n"), 0o644)
	if _, err := LoadRecipients(p); err == nil {
		t.Fatal("private key accepted as recipient")
	}
	os.WriteFile(p, []byte("# c\n"+id.Recipient().String()+" # laptop\n"), 0o644)
	rs, err := LoadRecipients(p)
	if err != nil || len(rs) != 1 || rs[0].Comment != "laptop" || rs[0].Kind != "x25519" {
		t.Fatalf("%v %+v", err, rs)
	}
}
