// Package vault wraps filippo.io/age for payload encryption and decryption,
// recipient files, and the path rules that keep plaintext out of the
// repository tree.
package vault

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"filippo.io/age"
	"filippo.io/age/agessh"
	"filippo.io/age/armor"
)

// Header prefixes of valid age files.
const (
	BinaryHeader  = "age-encryption.org/v1\n"
	ArmoredHeader = "-----BEGIN AGE ENCRYPTED FILE-----"
)

// PrivatePatterns match private key material that must never be published.
// The demo identity file is the only allowed exception.
var PrivatePatterns = []*regexp.Regexp{
	regexp.MustCompile(`AGE-SECRET-KEY-1[0-9A-Z]{20,}`),
	regexp.MustCompile(`AGE-SECRET-KEY-PQ-1[0-9A-Z]{20,}`),
	regexp.MustCompile(`AGE-PLUGIN-[0-9A-Z-]+-1[0-9A-Z]{20,}`),
	regexp.MustCompile(`-----BEGIN (OPENSSH|RSA|EC|DSA|PGP|ENCRYPTED)? ?PRIVATE KEY( BLOCK)?-----`),
}

// RecipientEntry is one parsed line of a recipients file.
type RecipientEntry struct {
	Line      int
	Value     string
	Comment   string
	Kind      string // x25519, hybrid-pq, ssh-ed25519, ssh-rsa
	Recipient age.Recipient
}

// ParseRecipient parses one recipient string.
func ParseRecipient(s string) (age.Recipient, string, error) {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, "age1pq1"):
		rs, err := age.ParseRecipients(strings.NewReader(s))
		if err != nil {
			return nil, "", err
		}
		return rs[0], "hybrid-pq", nil
	case strings.HasPrefix(s, "age1"):
		r, err := age.ParseX25519Recipient(s)
		return r, "x25519", err
	case strings.HasPrefix(s, "ssh-ed25519 "), strings.HasPrefix(s, "ssh-rsa "):
		r, err := agessh.ParseRecipient(s)
		return r, strings.Fields(s)[0], err
	}
	return nil, "", fmt.Errorf("unsupported recipient format %q", truncate(s, 16))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// LoadRecipients reads a recipients file: one public key per line, '#'
// comments, and an optional trailing "# comment" on a key line.
func LoadRecipients(path string) ([]RecipientEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []RecipientEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for _, p := range PrivatePatterns {
			if p.MatchString(line) {
				return nil, fmt.Errorf("%s:%d: private key material in a recipients file", path, n)
			}
		}
		val, comment, _ := strings.Cut(line, " #")
		r, kind, err := ParseRecipient(val)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, n, err)
		}
		out = append(out, RecipientEntry{Line: n, Value: strings.TrimSpace(val), Comment: strings.TrimSpace(comment), Kind: kind, Recipient: r})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: no recipients", path)
	}
	return out, nil
}

// Recipients extracts the age.Recipient values.
func Recipients(entries []RecipientEntry) []age.Recipient {
	rs := make([]age.Recipient, len(entries))
	for i, e := range entries {
		rs[i] = e.Recipient
	}
	return rs
}

// LoadIdentities reads an age identity file (native or hybrid) or an
// unencrypted SSH private key.
func LoadIdentities(path string) ([]age.Identity, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseIdentities(b)
}

// ParseIdentities parses identity file contents.
func ParseIdentities(b []byte) ([]age.Identity, error) {
	if bytes.Contains(b, []byte("PRIVATE KEY-----")) {
		id, err := agessh.ParseIdentity(b)
		if err != nil {
			return nil, fmt.Errorf("ssh identity: %w", err)
		}
		return []age.Identity{id}, nil
	}
	ids, err := age.ParseIdentities(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// Encrypt encrypts plaintext to recipients. armored selects ASCII armor.
func Encrypt(plaintext []byte, recipients []age.Recipient, armored bool) ([]byte, error) {
	if len(recipients) == 0 {
		return nil, errors.New("no recipients")
	}
	var buf bytes.Buffer
	var dst io.Writer = &buf
	var aw io.WriteCloser
	if armored {
		aw = armor.NewWriter(&buf)
		dst = aw
	}
	w, err := age.Encrypt(dst, recipients...)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	if aw != nil {
		if err := aw.Close(); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// Decrypt decrypts a binary or armored age file.
func Decrypt(ciphertext []byte, ids []age.Identity) ([]byte, error) {
	var src io.Reader = bytes.NewReader(ciphertext)
	if bytes.HasPrefix(bytes.TrimSpace(ciphertext), []byte(ArmoredHeader)) {
		src = armor.NewReader(bytes.NewReader(bytes.TrimSpace(ciphertext)))
	}
	r, err := age.Decrypt(src, ids...)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(r)
}

// IsAgeFile reports whether b starts with an age header.
func IsAgeFile(b []byte) bool {
	return bytes.HasPrefix(b, []byte(BinaryHeader)) || bytes.HasPrefix(bytes.TrimSpace(b), []byte(ArmoredHeader))
}

// SHA256Hex returns the hex SHA-256 of b.
func SHA256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// Inside reports whether path is inside dir (after resolving symlinks where possible).
func Inside(dir, path string) bool {
	d := resolve(dir)
	p := resolve(path)
	rel, err := filepath.Rel(d, p)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

func resolve(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	if r, err := filepath.EvalSymlinks(abs); err == nil {
		return r
	}
	// The file may not exist yet: resolve its parent.
	if r, err := filepath.EvalSymlinks(filepath.Dir(abs)); err == nil {
		return filepath.Join(r, filepath.Base(abs))
	}
	return abs
}

// ErrPlaintextInRepo is returned when a plaintext path is inside the repository.
var ErrPlaintextInRepo = errors.New("plaintext path is inside the repository; keep plaintext outside the tracked tree")

// WriteSecret writes plaintext with 0600 permissions, refusing repo paths.
func WriteSecret(repoRoot, path string, b []byte) error {
	if repoRoot != "" && Inside(repoRoot, path) {
		return ErrPlaintextInRepo
	}
	return os.WriteFile(path, b, 0o600)
}
