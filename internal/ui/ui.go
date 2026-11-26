// Package ui holds the shared palette, styles and table printer used by the
// CLI and the TUI. Colors follow the project palette: accent teal, danger red,
// silver secondary text, neutral greys, and aquamarine for at most one or two
// highlights per screen.
package ui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Palette.
var (
	Accent     = lipgloss.Color("#105c5c")
	AccentText = lipgloss.Color("#1f8a8a")
	Danger     = lipgloss.Color("#750f0f")
	DangerText = lipgloss.Color("#b83a3a")
	Silver     = lipgloss.Color("#c0c0c0")
	Aquamarine = lipgloss.Color("#7fffd4")
	Bg0        = lipgloss.Color("#0b0b0b")
	Bg1        = lipgloss.Color("#131313")
	Bg2        = lipgloss.Color("#1b1b1b")
	Line       = lipgloss.Color("#262626")
	White      = lipgloss.Color("#ffffff")
	Black      = lipgloss.Color("#000000")
	Grey       = lipgloss.Color("#8a8a8a")
)

// Styles.
var (
	Title     = lipgloss.NewStyle().Bold(true).Foreground(White).Background(Accent).Padding(0, 1)
	Heading   = lipgloss.NewStyle().Bold(true).Foreground(AccentText)
	Cmd       = lipgloss.NewStyle().Foreground(AccentText)
	Flag      = lipgloss.NewStyle().Foreground(Silver)
	Muted     = lipgloss.NewStyle().Foreground(Grey)
	Secondary = lipgloss.NewStyle().Foreground(Silver)
	OKStyle   = lipgloss.NewStyle().Foreground(AccentText).Bold(true)
	ErrStyle  = lipgloss.NewStyle().Foreground(DangerText).Bold(true)
	WarnStyle = lipgloss.NewStyle().Foreground(Silver).Bold(true)
	Highlight = lipgloss.NewStyle().Foreground(Black).Background(Aquamarine).Bold(true).Padding(0, 1)
	Badge     = lipgloss.NewStyle().Foreground(White).Background(Accent).Padding(0, 1)
	DangerBox = lipgloss.NewStyle().Foreground(White).Background(Danger).Padding(0, 1)
)

// colorEnabled tracks whether styling is active.
var colorEnabled = true

// SetColor configures color output: mode is auto, always, or never.
// NO_COLOR (any value) forces never unless mode is always.
func SetColor(mode string, noColor bool) {
	switch {
	case noColor:
		colorEnabled = false
	case mode == "always":
		colorEnabled = true
	case mode == "never":
		colorEnabled = false
	case os.Getenv("NO_COLOR") != "":
		colorEnabled = false
	default:
		colorEnabled = termenv.NewOutput(os.Stdout).ColorProfile() != termenv.Ascii
	}
	if colorEnabled {
		if mode == "always" {
			lipgloss.SetColorProfile(termenv.TrueColor)
		}
	} else {
		lipgloss.SetColorProfile(termenv.Ascii)
	}
}

// ColorEnabled reports whether output is styled.
func ColorEnabled() bool { return colorEnabled }

// Status renders a result word with the matching style.
func Status(s string) string {
	switch strings.ToLower(s) {
	case "ok", "pass", "verified", "stable", "yes":
		return OKStyle.Render(s)
	case "error", "fail", "failed", "deprecated", "broken":
		return ErrStyle.Render(s)
	case "warn", "unverified", "experimental", "draft", "skip":
		return WarnStyle.Render(s)
	}
	return s
}

// Table prints aligned columns. Cell widths ignore ANSI escape codes.
type Table struct {
	Headers []string
	Rows    [][]string
}

// Add appends a row.
func (t *Table) Add(cells ...string) { t.Rows = append(t.Rows, cells) }

// Fprint writes the table to w.
func (t *Table) Fprint(w io.Writer) {
	widths := make([]int, len(t.Headers))
	for i, h := range t.Headers {
		widths[i] = lipgloss.Width(h)
	}
	for _, r := range t.Rows {
		for i, c := range r {
			if i < len(widths) && lipgloss.Width(c) > widths[i] {
				widths[i] = lipgloss.Width(c)
			}
		}
	}
	pad := func(s string, n int) string { return s + strings.Repeat(" ", max(0, n-lipgloss.Width(s))) }
	var hdr []string
	for i, h := range t.Headers {
		hdr = append(hdr, Heading.Render(pad(h, widths[i])))
	}
	fmt.Fprintln(w, strings.TrimRight(strings.Join(hdr, "  "), " "))
	for _, r := range t.Rows {
		var cells []string
		for i, c := range r {
			if i == len(r)-1 {
				cells = append(cells, c)
			} else {
				cells = append(cells, pad(c, widths[i]))
			}
		}
		fmt.Fprintln(w, strings.Join(cells, "  "))
	}
}
