// Package classical implements historical ciphers for teaching. None of these
// ciphers provide modern confidentiality. They exist so lessons and adapted
// examples can be reproduced and checked automatically.
package classical

import (
	"errors"
	"strings"
	"unicode"
)

// Caesar shifts ASCII letters by shift positions, preserving case and leaving
// every other character unchanged. A negative shift decrypts.
func Caesar(text string, shift int) string {
	s := ((shift % 26) + 26) % 26
	var b strings.Builder
	for _, r := range text {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune('A' + (r-'A'+rune(s))%26)
		case r >= 'a' && r <= 'z':
			b.WriteRune('a' + (r-'a'+rune(s))%26)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func vigenere(text, key string, sign int) (string, error) {
	var k []int
	for _, r := range strings.ToUpper(key) {
		if r >= 'A' && r <= 'Z' {
			k = append(k, int(r-'A'))
		}
	}
	if len(k) == 0 {
		return "", errors.New("vigenere key must contain at least one letter")
	}
	var b strings.Builder
	i := 0
	for _, r := range text {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune('A' + rune((int(r-'A')+sign*k[i%len(k)]+26)%26))
			i++
		case r >= 'a' && r <= 'z':
			b.WriteRune('a' + rune((int(r-'a')+sign*k[i%len(k)]+26)%26))
			i++
		default:
			b.WriteRune(r)
		}
	}
	return b.String(), nil
}

// VigenereEncrypt encrypts letters with a repeating key. The key index only
// advances on letters.
func VigenereEncrypt(text, key string) (string, error) { return vigenere(text, key, 1) }

// VigenereDecrypt reverses VigenereEncrypt.
func VigenereDecrypt(text, key string) (string, error) { return vigenere(text, key, -1) }

// playfairSquare builds the 5x5 key square. J is merged into I.
func playfairSquare(key string) [25]rune {
	var sq [25]rune
	seen := map[rune]bool{}
	n := 0
	add := func(r rune) {
		r = unicode.ToUpper(r)
		if r == 'J' {
			r = 'I'
		}
		if r < 'A' || r > 'Z' || seen[r] {
			return
		}
		seen[r] = true
		sq[n] = r
		n++
	}
	for _, r := range key {
		add(r)
	}
	for r := 'A'; r <= 'Z'; r++ {
		add(r)
	}
	return sq
}

// PlayfairDigraphs prepares plaintext: letters only, upper case, J->I, an X
// between doubled letters in the same pair, and a trailing X for odd length.
func PlayfairDigraphs(text string) []string {
	var letters []rune
	for _, r := range strings.ToUpper(text) {
		if r == 'J' {
			r = 'I'
		}
		if r >= 'A' && r <= 'Z' {
			letters = append(letters, r)
		}
	}
	var out []string
	for i := 0; i < len(letters); {
		a := letters[i]
		if i+1 >= len(letters) {
			out = append(out, string([]rune{a, 'X'}))
			break
		}
		b := letters[i+1]
		if a == b {
			out = append(out, string([]rune{a, 'X'}))
			i++
			continue
		}
		out = append(out, string([]rune{a, b}))
		i += 2
	}
	return out
}

func playfair(pairs []string, key string, step int) string {
	sq := playfairSquare(key)
	pos := map[rune]int{}
	for i, r := range sq {
		pos[r] = i
	}
	var b strings.Builder
	for _, p := range pairs {
		rs := []rune(p)
		i, j := pos[rs[0]], pos[rs[1]]
		ri, ci, rj, cj := i/5, i%5, j/5, j%5
		switch {
		case ri == rj:
			b.WriteRune(sq[ri*5+(ci+step+5)%5])
			b.WriteRune(sq[rj*5+(cj+step+5)%5])
		case ci == cj:
			b.WriteRune(sq[((ri+step+5)%5)*5+ci])
			b.WriteRune(sq[((rj+step+5)%5)*5+cj])
		default:
			b.WriteRune(sq[ri*5+cj])
			b.WriteRune(sq[rj*5+ci])
		}
	}
	return b.String()
}

// PlayfairEncrypt encrypts text with the classic Playfair rules.
func PlayfairEncrypt(text, key string) string { return playfair(PlayfairDigraphs(text), key, 1) }

// PlayfairDecrypt decrypts an even-length Playfair ciphertext. Filler X letters stay in the output.
func PlayfairDecrypt(ciphertext, key string) (string, error) {
	var letters []rune
	for _, r := range strings.ToUpper(ciphertext) {
		if r >= 'A' && r <= 'Z' {
			letters = append(letters, r)
		}
	}
	if len(letters)%2 != 0 {
		return "", errors.New("playfair ciphertext must have an even number of letters")
	}
	var pairs []string
	for i := 0; i < len(letters); i += 2 {
		pairs = append(pairs, string(letters[i:i+2]))
	}
	return playfair(pairs, key, -1), nil
}
