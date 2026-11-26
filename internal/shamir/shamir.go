// Package shamir implements byte-wise Shamir's Secret Sharing over GF(2^8)
// with the AES reduction polynomial x^8 + x^4 + x^3 + x + 1 (0x11b).
//
// Share format: one byte x coordinate (1..255) followed by len(secret) bytes of
// y values, one polynomial per secret byte. This is a teaching implementation;
// it has not been audited and does not defend against malicious shares.
package shamir

import (
	"crypto/rand"
	"errors"
	"io"
)

func gfMul(a, b byte) byte {
	var p byte
	for b > 0 {
		if b&1 == 1 {
			p ^= a
		}
		hi := a & 0x80
		a <<= 1
		if hi != 0 {
			a ^= 0x1b
		}
		b >>= 1
	}
	return p
}

func gfInv(a byte) byte {
	// a^254 = a^-1 in GF(2^8) for a != 0.
	r := byte(1)
	for i := 0; i < 254; i++ {
		r = gfMul(r, a)
	}
	return r
}

// Split creates n shares of secret; any threshold of them reconstruct it.
func Split(secret []byte, n, threshold int) ([][]byte, error) {
	return SplitWithRand(rand.Reader, secret, n, threshold)
}

// SplitWithRand is Split with an explicit randomness source (tests and challenge generation).
func SplitWithRand(r io.Reader, secret []byte, n, threshold int) ([][]byte, error) {
	if threshold < 2 || n < threshold || n > 255 {
		return nil, errors.New("need 2 <= threshold <= n <= 255")
	}
	if len(secret) == 0 {
		return nil, errors.New("secret must not be empty")
	}
	shares := make([][]byte, n)
	for i := range shares {
		shares[i] = make([]byte, len(secret)+1)
		shares[i][0] = byte(i + 1)
	}
	coef := make([]byte, threshold-1)
	for bi, s := range secret {
		if _, err := io.ReadFull(r, coef); err != nil {
			return nil, err
		}
		for i := range shares {
			x := byte(i + 1)
			// Horner evaluation of s + c1 x + ... + c(t-1) x^(t-1).
			y := byte(0)
			for k := len(coef) - 1; k >= 0; k-- {
				y = gfMul(y, x) ^ coef[k]
			}
			y = gfMul(y, x) ^ s
			shares[i][bi+1] = y
		}
	}
	return shares, nil
}

// Combine reconstructs the secret with Lagrange interpolation at x = 0.
// It returns garbage, not an error, when fewer than threshold shares are given;
// that is a property of the scheme, not a bug.
func Combine(shares [][]byte) ([]byte, error) {
	if len(shares) < 2 {
		return nil, errors.New("need at least two shares")
	}
	l := len(shares[0])
	seen := map[byte]bool{}
	for _, s := range shares {
		if len(s) != l || l < 2 {
			return nil, errors.New("shares must have equal length >= 2")
		}
		if s[0] == 0 || seen[s[0]] {
			return nil, errors.New("share x coordinates must be unique and non-zero")
		}
		seen[s[0]] = true
	}
	secret := make([]byte, l-1)
	for bi := range secret {
		var acc byte
		for i, si := range shares {
			num, den := byte(1), byte(1)
			for j, sj := range shares {
				if i == j {
					continue
				}
				num = gfMul(num, sj[0])
				den = gfMul(den, sj[0]^si[0])
			}
			acc ^= gfMul(si[bi+1], gfMul(num, gfInv(den)))
		}
		secret[bi] = acc
	}
	return secret, nil
}
