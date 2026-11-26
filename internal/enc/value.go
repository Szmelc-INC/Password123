// Package enc decodes explicitly encoded values used in public vector and
// challenge files. Every value must name exactly one encoding so that a reader
// never has to guess whether "00ff" is hex, text, or something else.
package enc

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// Value is a single encoded value. Exactly one field must be set.
type Value struct {
	Hex        *string `yaml:"hex,omitempty" json:"hex,omitempty"`
	UTF8       *string `yaml:"utf8,omitempty" json:"utf8,omitempty"`
	Base64     *string `yaml:"base64,omitempty" json:"base64,omitempty"`
	Int        *int64  `yaml:"int,omitempty" json:"int,omitempty"`
	Dec        *string `yaml:"dec,omitempty" json:"dec,omitempty"`
	Pattern251 *int    `yaml:"pattern251,omitempty" json:"pattern251,omitempty"`
}

// Encoding returns the name of the encoding that is set.
func (v Value) Encoding() (string, error) {
	var set []string
	if v.Hex != nil {
		set = append(set, "hex")
	}
	if v.UTF8 != nil {
		set = append(set, "utf8")
	}
	if v.Base64 != nil {
		set = append(set, "base64")
	}
	if v.Int != nil {
		set = append(set, "int")
	}
	if v.Dec != nil {
		set = append(set, "dec")
	}
	if v.Pattern251 != nil {
		set = append(set, "pattern251")
	}
	if len(set) != 1 {
		return "", fmt.Errorf("value must use exactly one encoding, found %d (%s)", len(set), strings.Join(set, ","))
	}
	return set[0], nil
}

// Bytes decodes the value as a byte string.
func (v Value) Bytes() ([]byte, error) {
	e, err := v.Encoding()
	if err != nil {
		return nil, err
	}
	switch e {
	case "hex":
		return DecodeHex(*v.Hex)
	case "utf8":
		return []byte(*v.UTF8), nil
	case "base64":
		return base64.StdEncoding.DecodeString(strings.TrimSpace(*v.Base64))
	case "pattern251":
		n := *v.Pattern251
		if n < 0 {
			return nil, errors.New("pattern251 length must be >= 0")
		}
		b := make([]byte, n)
		for i := range b {
			b[i] = byte(i % 251)
		}
		return b, nil
	}
	return nil, fmt.Errorf("encoding %q is not a byte string", e)
}

// String decodes the value as UTF-8 text.
func (v Value) String() (string, error) {
	b, err := v.Bytes()
	return string(b), err
}

// Integer decodes the value as an int64.
func (v Value) Integer() (int64, error) {
	e, err := v.Encoding()
	if err != nil {
		return 0, err
	}
	if e != "int" {
		return 0, fmt.Errorf("encoding %q is not an integer", e)
	}
	return *v.Int, nil
}

// Big decodes the value as an arbitrary-precision integer (hex big-endian or decimal).
func (v Value) Big() (*big.Int, error) {
	e, err := v.Encoding()
	if err != nil {
		return nil, err
	}
	switch e {
	case "dec":
		n, ok := new(big.Int).SetString(strings.Join(strings.Fields(*v.Dec), ""), 10)
		if !ok {
			return nil, errors.New("invalid decimal integer")
		}
		return n, nil
	case "int":
		return big.NewInt(*v.Int), nil
	case "hex":
		b, err := DecodeHex(*v.Hex)
		if err != nil {
			return nil, err
		}
		return new(big.Int).SetBytes(b), nil
	}
	return nil, fmt.Errorf("encoding %q is not an integer", e)
}

// DecodeHex decodes hex that may contain whitespace or ':' separators, as
// printed in RFC hexdumps.
func DecodeHex(s string) ([]byte, error) {
	clean := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r', ':':
			return -1
		}
		return r
	}, s)
	return hex.DecodeString(clean)
}

// Hex returns a hex Value.
func Hex(b []byte) Value { s := hex.EncodeToString(b); return Value{Hex: &s} }

// Text returns a UTF-8 Value.
func Text(s string) Value { return Value{UTF8: &s} }

// Num returns an integer Value.
func Num(n int64) Value { return Value{Int: &n} }
