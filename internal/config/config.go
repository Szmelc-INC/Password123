// Package config loads the optional p123 config file and resolves the
// repository root. Precedence: flags > environment > config file > defaults.
package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is $XDG_CONFIG_HOME/password123/config.yaml.
type Config struct {
	// Repo is the repository checkout used when not inside one.
	Repo string `yaml:"repo,omitempty" json:"repo,omitempty"`
	// Identity is the private age identity file used for decrypt and rotate.
	Identity string `yaml:"identity,omitempty" json:"identity,omitempty"`
	// Recipients overrides recipients/maintainers.txt.
	Recipients string `yaml:"recipients,omitempty" json:"recipients,omitempty"`
	// Color is auto, always, or never.
	Color string `yaml:"color,omitempty" json:"color,omitempty"`
	// Armor writes ASCII-armored payloads when true.
	Armor bool `yaml:"armor,omitempty" json:"armor,omitempty"`
}

// Dir returns the config directory.
func Dir() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "password123")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "password123")
}

// DefaultPath returns the default config file path.
func DefaultPath() string { return filepath.Join(Dir(), "config.yaml") }

// DefaultIdentity is the identity path used when nothing else is configured.
func DefaultIdentity() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "age", "password123.txt")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "age", "password123.txt")
}

// Load reads path; a missing file yields an empty config.
func Load(path string) (*Config, error) {
	var c Config
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &c, nil
	}
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil && err.Error() != "EOF" {
		return nil, err
	}
	c.Repo = Expand(c.Repo)
	c.Identity = Expand(c.Identity)
	c.Recipients = Expand(c.Recipients)
	return &c, nil
}

// Expand replaces a leading ~/ with the home directory.
func Expand(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

// IsRepo reports whether dir looks like a Password123 checkout.
func IsRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "schemas", "lesson.schema.json"))
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, "curriculum"))
	return err == nil
}

// FindRepo walks up from start looking for a checkout.
func FindRepo(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if IsRepo(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("not inside a Password123 checkout (use --repo, P123_REPO, or repo: in the config file)")
		}
		dir = parent
	}
}

// Template is written by `p123 config init`.
const Template = `# p123 configuration. Flags and environment variables override these values.
# repo: ~/src/Password123          # checkout used when the shell is not inside one (env: P123_REPO)
# identity: ~/.config/age/password123.txt   # private age identity, never inside the repo (env: P123_IDENTITY)
# recipients: ""                   # defaults to <repo>/recipients/maintainers.txt
# color: auto                      # auto | always | never (NO_COLOR is honoured)
# armor: false                     # write ASCII-armored payloads
`
