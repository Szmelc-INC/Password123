package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelFlow(t *testing.T) {
	root, _ := filepath.Abs("../..")
	m, err := newModel(Options{Root: root, Identity: filepath.Join(root, "demo", "demo-identity.txt"), Glamour: []byte(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	if v := m.View(); !strings.Contains(v, "Password123") || !strings.Contains(v, "locked") {
		t.Fatalf("unexpected first view")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if !strings.Contains(m.View(), "Keybindings") {
		t.Fatal("help overlay missing")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m.run("open 02-aes-256-gcm")
	if m.current != "02-aes-256-gcm" {
		t.Fatalf("open failed: %s", m.current)
	}
	m.run("verify")
	if !m.statusOK || !strings.Contains(m.status, "vectors pass") {
		t.Fatalf("verify: %s", m.status)
	}
	// The demo identity must not open a real payload.
	m.run("decrypt 02-aes-256-gcm")
	if m.statusOK || m.plaintext {
		t.Fatal("demo identity decrypted a real payload")
	}
	m.run("module 06")
	if len(m.list.Items()) != 3 {
		t.Fatalf("module filter: %d items", len(m.list.Items()))
	}
	m.lock()
	if m.plaintext || m.extra != "" {
		t.Fatal("lock did not clear")
	}
}
