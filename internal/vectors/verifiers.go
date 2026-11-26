package vectors

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/mldsa"
	"crypto/mlkem"
	"crypto/mlkem/mlkemtest"
	"crypto/pbkdf2"
	"crypto/rc4"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha3"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"math/big"
	"strings"

	"filippo.io/age"
	"github.com/Szmelc-INC/Password123/internal/classical"
	"github.com/Szmelc-INC/Password123/internal/enc"
	"github.com/Szmelc-INC/Password123/internal/shamir"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"lukechampine.com/blake3"
)

// args gives verifiers typed access to a vector's fields and collects the
// first decoding error, so verifiers stay short and fail closed.
type args struct {
	v   Vector
	err error
}

func (a *args) get(section, name string) (enc.Value, bool) {
	var m map[string]enc.Value
	switch section {
	case "params":
		m = a.v.Params
	case "inputs":
		m = a.v.Inputs
	case "expected":
		m = a.v.Expected
	}
	val, ok := m[name]
	if !ok && a.err == nil {
		a.err = fmt.Errorf("missing %s.%s", section, name)
	}
	return val, ok
}

func (a *args) bytes(section, name string) []byte {
	val, ok := a.get(section, name)
	if !ok {
		return nil
	}
	b, err := val.Bytes()
	if err != nil && a.err == nil {
		a.err = fmt.Errorf("%s.%s: %w", section, name, err)
	}
	return b
}

func (a *args) text(section, name string) string { return string(a.bytes(section, name)) }

func (a *args) int(section, name string) int64 {
	val, ok := a.get(section, name)
	if !ok {
		return 0
	}
	n, err := val.Integer()
	if err != nil && a.err == nil {
		a.err = fmt.Errorf("%s.%s: %w", section, name, err)
	}
	return n
}

func (a *args) big(section, name string) *big.Int {
	val, ok := a.get(section, name)
	if !ok {
		return new(big.Int)
	}
	n, err := val.Big()
	if err != nil && a.err == nil {
		a.err = fmt.Errorf("%s.%s: %w", section, name, err)
	}
	if n == nil {
		return new(big.Int)
	}
	return n
}

func mismatch(name string, got, want []byte) error {
	return fmt.Errorf("%s mismatch: got %s, want %s", name, hex.EncodeToString(got), hex.EncodeToString(want))
}

func check(name string, got, want []byte) error {
	if !bytes.Equal(got, want) {
		return mismatch(name, got, want)
	}
	return nil
}

func checkText(name, got, want string) error {
	if got != want {
		return fmt.Errorf("%s mismatch: got %q, want %q", name, got, want)
	}
	return nil
}

var registry = map[string]func(*args) error{
	"caesar":                 verifyCaesar,
	"vigenere":               verifyVigenere,
	"playfair":               verifyPlayfair,
	"rc4-keystream":          verifyRC4,
	"aes-cbc":                verifyAESCBC,
	"aes-gcm":                verifyAESGCM,
	"chacha20-poly1305":      verifyChaCha20Poly1305,
	"rsa-oaep-decrypt":       verifyRSAOAEPDecrypt,
	"modp-safe-prime":        verifyMODPSafePrime,
	"x25519":                 verifyX25519,
	"x25519-exchange":        verifyX25519Exchange,
	"ed25519":                verifyEd25519,
	"sha-256":                verifySHA256,
	"blake3":                 verifyBLAKE3,
	"hmac-sha-256":           verifyHMACSHA256,
	"pbkdf2-hmac-sha-256":    verifyPBKDF2,
	"argon2id":               verifyArgon2id,
	"shamir-gf256-property":  verifyShamirProperty,
	"ml-kem-768-accumulated": verifyMLKEMAccumulated,
	"ml-dsa-65-accumulated":  verifyMLDSAAccumulated,
	"age-hybrid-roundtrip":   verifyAgeHybridRoundTrip,
}

func verifyCaesar(a *args) error {
	pt, shift, want := a.text("inputs", "plaintext"), a.int("params", "shift"), a.text("expected", "ciphertext")
	if a.err != nil {
		return a.err
	}
	if err := checkText("ciphertext", classical.Caesar(pt, int(shift)), want); err != nil {
		return err
	}
	return checkText("round trip", classical.Caesar(want, -int(shift)), pt)
}

func verifyVigenere(a *args) error {
	pt, key, want := a.text("inputs", "plaintext"), a.text("inputs", "key"), a.text("expected", "ciphertext")
	if a.err != nil {
		return a.err
	}
	got, err := classical.VigenereEncrypt(pt, key)
	if err != nil {
		return err
	}
	if err := checkText("ciphertext", got, want); err != nil {
		return err
	}
	back, err := classical.VigenereDecrypt(want, key)
	if err != nil {
		return err
	}
	return checkText("round trip", back, pt)
}

func verifyPlayfair(a *args) error {
	pt, key, want := a.text("inputs", "plaintext"), a.text("inputs", "key"), a.text("expected", "ciphertext")
	if a.err != nil {
		return a.err
	}
	got := classical.PlayfairEncrypt(pt, key)
	if err := checkText("ciphertext", got, want); err != nil {
		return err
	}
	back, err := classical.PlayfairDecrypt(want, key)
	if err != nil {
		return err
	}
	return checkText("round trip", back, strings.Join(classical.PlayfairDigraphs(pt), ""))
}

func verifyRC4(a *args) error {
	key, offset, want := a.bytes("inputs", "key"), a.int("params", "offset"), a.bytes("expected", "keystream")
	if a.err != nil {
		return a.err
	}
	c, err := rc4.NewCipher(key)
	if err != nil {
		return err
	}
	buf := make([]byte, int(offset)+len(want))
	c.XORKeyStream(buf, buf)
	return check("keystream", buf[offset:], want)
}

func verifyAESCBC(a *args) error {
	key, iv, pt, want := a.bytes("inputs", "key"), a.bytes("inputs", "iv"), a.bytes("inputs", "plaintext"), a.bytes("expected", "ciphertext")
	if a.err != nil {
		return a.err
	}
	if len(pt)%aes.BlockSize != 0 {
		return errors.New("CBC vector plaintext must be block-aligned (the source uses no padding)")
	}
	blk, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	got := make([]byte, len(pt))
	cipher.NewCBCEncrypter(blk, iv).CryptBlocks(got, pt)
	if err := check("ciphertext", got, want); err != nil {
		return err
	}
	back := make([]byte, len(want))
	cipher.NewCBCDecrypter(blk, iv).CryptBlocks(back, want)
	return check("round trip", back, pt)
}

// verifyAEAD checks seal, open, and three tampering cases that must fail.
func verifyAEAD(a *args, aead cipher.AEAD) error {
	nonce, aad, pt := a.bytes("inputs", "nonce"), a.bytes("inputs", "aad"), a.bytes("inputs", "plaintext")
	wantCT, wantTag := a.bytes("expected", "ciphertext"), a.bytes("expected", "tag")
	if a.err != nil {
		return a.err
	}
	if len(nonce) != aead.NonceSize() {
		return fmt.Errorf("nonce must be %d bytes, got %d", aead.NonceSize(), len(nonce))
	}
	sealed := aead.Seal(nil, nonce, pt, aad)
	ct, tag := sealed[:len(sealed)-aead.Overhead()], sealed[len(sealed)-aead.Overhead():]
	if err := check("ciphertext", ct, wantCT); err != nil {
		return err
	}
	if err := check("tag", tag, wantTag); err != nil {
		return err
	}
	opened, err := aead.Open(nil, nonce, sealed, aad)
	if err != nil {
		return fmt.Errorf("open failed: %w", err)
	}
	if err := check("round trip", opened, pt); err != nil {
		return err
	}
	tampered := bytes.Clone(sealed)
	tampered[len(tampered)-1] ^= 0x01
	if _, err := aead.Open(nil, nonce, tampered, aad); err == nil {
		return errors.New("tampered tag was accepted")
	}
	if len(ct) > 0 {
		tampered = bytes.Clone(sealed)
		tampered[0] ^= 0x80
		if _, err := aead.Open(nil, nonce, tampered, aad); err == nil {
			return errors.New("tampered ciphertext was accepted")
		}
	}
	if _, err := aead.Open(nil, nonce, sealed, append(bytes.Clone(aad), 0x00)); err == nil {
		return errors.New("modified associated data was accepted")
	}
	return nil
}

func verifyAESGCM(a *args) error {
	key := a.bytes("inputs", "key")
	if a.err != nil {
		return a.err
	}
	blk, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(blk)
	if err != nil {
		return err
	}
	return verifyAEAD(a, aead)
}

func verifyChaCha20Poly1305(a *args) error {
	key := a.bytes("inputs", "key")
	if a.err != nil {
		return a.err
	}
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return err
	}
	return verifyAEAD(a, aead)
}

func verifyRSAOAEPDecrypt(a *args) error {
	n, e, d := a.big("inputs", "modulus"), a.big("inputs", "public_exponent"), a.big("inputs", "private_exponent")
	p, q := a.big("inputs", "prime1"), a.big("inputs", "prime2")
	ct, want := a.bytes("inputs", "ciphertext"), a.bytes("expected", "message")
	hashName := a.text("params", "hash")
	if a.err != nil {
		return a.err
	}
	var h hash.Hash
	switch hashName {
	case "SHA-1":
		h = sha1.New()
	case "SHA-256":
		h = sha256.New()
	default:
		return fmt.Errorf("unsupported OAEP hash %q", hashName)
	}
	key := &rsa.PrivateKey{PublicKey: rsa.PublicKey{N: n, E: int(e.Int64())}, D: d, Primes: []*big.Int{p, q}}
	if err := key.Validate(); err != nil {
		return fmt.Errorf("key validation: %w", err)
	}
	key.Precompute()
	got, err := rsa.DecryptOAEP(h, nil, key, ct, nil)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}
	if err := check("message", got, want); err != nil {
		return err
	}
	bad := bytes.Clone(ct)
	bad[len(bad)-1] ^= 0x01
	if _, err := rsa.DecryptOAEP(h, nil, key, bad, nil); err == nil {
		return errors.New("modified ciphertext decrypted without error")
	}
	return nil
}

func verifyMODPSafePrime(a *args) error {
	p, g, bits := a.big("inputs", "prime"), a.big("inputs", "generator"), a.int("params", "bits")
	if a.err != nil {
		return a.err
	}
	if p.BitLen() != int(bits) {
		return fmt.Errorf("prime has %d bits, want %d", p.BitLen(), bits)
	}
	if !p.ProbablyPrime(64) {
		return errors.New("p is not prime")
	}
	q := new(big.Int).Rsh(new(big.Int).Sub(p, big.NewInt(1)), 1)
	if !q.ProbablyPrime(64) {
		return errors.New("(p-1)/2 is not prime, so p is not a safe prime")
	}
	// g generates the subgroup of order q iff g^q == 1 mod p (and g != 1).
	if g.Cmp(big.NewInt(1)) <= 0 || new(big.Int).Exp(g, q, p).Cmp(big.NewInt(1)) != 0 {
		return errors.New("generator does not generate the prime-order subgroup")
	}
	return nil
}

func verifyX25519(a *args) error {
	scalar, u, want := a.bytes("inputs", "scalar"), a.bytes("inputs", "u"), a.bytes("expected", "output")
	if a.err != nil {
		return a.err
	}
	got, err := curve25519.X25519(scalar, u)
	if err != nil {
		return err
	}
	return check("output", got, want)
}

func verifyX25519Exchange(a *args) error {
	ap, bp := a.bytes("inputs", "alice_private"), a.bytes("inputs", "bob_private")
	apub, bpub, k := a.bytes("expected", "alice_public"), a.bytes("expected", "bob_public"), a.bytes("expected", "shared_secret")
	if a.err != nil {
		return a.err
	}
	gotA, err := curve25519.X25519(ap, curve25519.Basepoint)
	if err != nil {
		return err
	}
	gotB, err := curve25519.X25519(bp, curve25519.Basepoint)
	if err != nil {
		return err
	}
	if err := check("alice_public", gotA, apub); err != nil {
		return err
	}
	if err := check("bob_public", gotB, bpub); err != nil {
		return err
	}
	k1, err := curve25519.X25519(ap, bpub)
	if err != nil {
		return err
	}
	k2, err := curve25519.X25519(bp, apub)
	if err != nil {
		return err
	}
	if err := check("shared_secret (alice)", k1, k); err != nil {
		return err
	}
	return check("shared_secret (bob)", k2, k)
}

func verifyEd25519(a *args) error {
	seed, msg := a.bytes("inputs", "secret_key"), a.bytes("inputs", "message")
	wantPub, wantSig := a.bytes("expected", "public_key"), a.bytes("expected", "signature")
	if a.err != nil {
		return a.err
	}
	if len(seed) != ed25519.SeedSize {
		return fmt.Errorf("secret key must be a %d-byte seed", ed25519.SeedSize)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	if err := check("public_key", pub, wantPub); err != nil {
		return err
	}
	if err := check("signature", ed25519.Sign(priv, msg), wantSig); err != nil {
		return err
	}
	if !ed25519.Verify(pub, msg, wantSig) {
		return errors.New("signature does not verify")
	}
	if ed25519.Verify(pub, append(bytes.Clone(msg), 0x00), wantSig) {
		return errors.New("signature verified for a modified message")
	}
	return nil
}

func verifySHA256(a *args) error {
	msg, want := a.bytes("inputs", "message"), a.bytes("expected", "digest")
	if a.err != nil {
		return a.err
	}
	sum := sha256.Sum256(msg)
	return check("digest", sum[:], want)
}

func verifyBLAKE3(a *args) error {
	msg, want := a.bytes("inputs", "message"), a.bytes("expected", "hash")
	if a.err != nil {
		return a.err
	}
	h := blake3.New(len(want), nil)
	h.Write(msg)
	return check("hash", h.Sum(nil), want)
}

func verifyHMACSHA256(a *args) error {
	key, data, want := a.bytes("inputs", "key"), a.bytes("inputs", "data"), a.bytes("expected", "mac")
	if a.err != nil {
		return a.err
	}
	m := hmac.New(sha256.New, key)
	m.Write(data)
	got := m.Sum(nil)
	if err := check("mac", got[:len(want)], want); err != nil {
		return err
	}
	if !hmac.Equal(got[:len(want)], want) {
		return errors.New("constant-time comparison disagrees")
	}
	return nil
}

func verifyPBKDF2(a *args) error {
	pw, salt := a.text("inputs", "password"), a.bytes("inputs", "salt")
	iter, dkLen, want := a.int("params", "iterations"), a.int("params", "dk_len"), a.bytes("expected", "derived_key")
	if a.err != nil {
		return a.err
	}
	got, err := pbkdf2.Key(sha256.New, pw, salt, int(iter), int(dkLen))
	if err != nil {
		return err
	}
	return check("derived_key", got, want)
}

func verifyArgon2id(a *args) error {
	pw, salt := a.bytes("inputs", "password"), a.bytes("inputs", "salt")
	t, m, p := a.int("params", "time_cost"), a.int("params", "memory_kib"), a.int("params", "parallelism")
	tagLen, want := a.int("params", "tag_len"), a.bytes("expected", "tag")
	encoded := ""
	if _, ok := a.v.Expected["encoded"]; ok {
		encoded = a.text("expected", "encoded")
	}
	if a.err != nil {
		return a.err
	}
	got := argon2.IDKey(pw, salt, uint32(t), uint32(m), uint8(p), uint32(tagLen))
	if err := check("tag", got, want); err != nil {
		return err
	}
	if encoded != "" {
		b64 := base64.RawStdEncoding
		gotEnc := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, m, t, p, b64.EncodeToString(salt), b64.EncodeToString(got))
		return checkText("encoded", gotEnc, encoded)
	}
	return nil
}

func verifyShamirProperty(a *args) error {
	secret := a.bytes("inputs", "secret")
	n, t := a.int("params", "shares"), a.int("params", "threshold")
	if a.err != nil {
		return a.err
	}
	shares, err := shamir.Split(secret, int(n), int(t))
	if err != nil {
		return err
	}
	// Every contiguous window of t shares must reconstruct the secret.
	for i := 0; i+int(t) <= len(shares); i++ {
		got, err := shamir.Combine(shares[i : i+int(t)])
		if err != nil {
			return err
		}
		if err := check(fmt.Sprintf("reconstruction from shares %d..%d", i+1, i+int(t)), got, secret); err != nil {
			return err
		}
	}
	// t-1 shares almost never reconstruct the secret (probability 256^-len).
	got, err := shamir.Combine(shares[:t-1])
	if err == nil && bytes.Equal(got, secret) && len(secret) >= 8 {
		return errors.New("threshold-1 shares reconstructed the secret")
	}
	return nil
}

// verifyMLKEMAccumulated reproduces the C2SP CCTV accumulated ML-KEM-768 test:
// n deterministic key pairs and encapsulations drawn from a SHAKE128 stream,
// all outputs absorbed into a second SHAKE128 whose first 32 bytes are checked.
func verifyMLKEMAccumulated(a *args) error {
	n, want := a.int("params", "iterations"), a.bytes("expected", "accumulated")
	if a.err != nil {
		return a.err
	}
	s, o := sha3.NewSHAKE128(), sha3.NewSHAKE128()
	seed := make([]byte, mlkem.SeedSize)
	msg := make([]byte, 32)
	ct1 := make([]byte, mlkem.CiphertextSize768)
	for i := int64(0); i < n; i++ {
		s.Read(seed)
		dk, err := mlkem.NewDecapsulationKey768(seed)
		if err != nil {
			return err
		}
		ek := dk.EncapsulationKey()
		o.Write(ek.Bytes())
		s.Read(msg)
		k, ct, err := mlkemtest.Encapsulate768(ek, msg)
		if err != nil {
			return err
		}
		o.Write(ct)
		o.Write(k)
		kk, err := dk.Decapsulate(ct)
		if err != nil {
			return err
		}
		if !bytes.Equal(kk, k) {
			return fmt.Errorf("iteration %d: decapsulated key differs", i)
		}
		s.Read(ct1)
		k1, err := dk.Decapsulate(ct1)
		if err != nil {
			return err
		}
		o.Write(k1)
	}
	got := make([]byte, 32)
	o.Read(got)
	return check("accumulated", got, want)
}

// verifyMLDSAAccumulated reproduces the C2SP CCTV accumulated ML-DSA-65 test
// using deterministic signing of the empty message.
func verifyMLDSAAccumulated(a *args) error {
	n, want := a.int("params", "iterations"), a.bytes("expected", "accumulated")
	if a.err != nil {
		return a.err
	}
	params := mldsa.MLDSA65()
	s, o := sha3.NewSHAKE128(), sha3.NewSHAKE128()
	seed := make([]byte, mldsa.PrivateKeySize)
	msg := []byte{}
	for i := int64(0); i < n; i++ {
		s.Read(seed)
		dk, err := mldsa.NewPrivateKey(params, seed)
		if err != nil {
			return err
		}
		pk := dk.PublicKey().Bytes()
		o.Write(pk)
		sig, err := dk.SignDeterministic(msg, nil)
		if err != nil {
			return err
		}
		o.Write(sig)
		if err := mldsa.Verify(dk.PublicKey(), msg, sig, nil); err != nil {
			return fmt.Errorf("iteration %d: %w", i, err)
		}
	}
	got := make([]byte, 32)
	o.Read(got)
	return check("accumulated", got, want)
}

// verifyAgeHybridRoundTrip checks that a fresh age hybrid (ML-KEM-768 +
// X25519) identity round-trips a message and that an unrelated identity fails.
func verifyAgeHybridRoundTrip(a *args) error {
	msg := a.bytes("inputs", "message")
	if a.err != nil {
		return a.err
	}
	id, err := age.GenerateHybridIdentity()
	if err != nil {
		return err
	}
	if !strings.HasPrefix(id.Recipient().String(), "age1pq1") {
		return fmt.Errorf("unexpected hybrid recipient prefix in %q", id.Recipient().String()[:10])
	}
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, id.Recipient())
	if err != nil {
		return err
	}
	w.Write(msg)
	if err := w.Close(); err != nil {
		return err
	}
	r, err := age.Decrypt(bytes.NewReader(buf.Bytes()), id)
	if err != nil {
		return err
	}
	got, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if err := check("round trip", got, msg); err != nil {
		return err
	}
	other, _ := age.GenerateHybridIdentity()
	if _, err := age.Decrypt(bytes.NewReader(buf.Bytes()), other); err == nil {
		return errors.New("an unrelated identity decrypted the file")
	}
	return nil
}
