// Package render turns public lesson data into Markdown for the CLI and TUI.
package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Szmelc-INC/Password123/internal/manifest"
	"github.com/Szmelc-INC/Password123/internal/vectors"
)

// LessonMarkdown renders one lesson. vfiles may be nil.
func LessonMarkdown(l manifest.Lesson, vfiles []*vectors.File) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", l.Title)
	fmt.Fprintf(&b, "`%s` · module `%s` · %s · status **%s**", l.ID, l.Module, l.Difficulty, l.Status)
	if l.RecommendedForNew {
		b.WriteString(" · recommended for new systems")
	} else {
		b.WriteString(" · **not** recommended for new systems")
	}
	fmt.Fprintf(&b, "\n\n> %s\n\n", l.Summary)
	section := func(title, body string) {
		if strings.TrimSpace(body) != "" {
			fmt.Fprintf(&b, "## %s\n\n%s\n\n", title, strings.TrimSpace(body))
		}
	}
	section("What it does", l.Guide.What)
	section("What problem it solves", l.Guide.Problem)
	section("Inputs and outputs", l.Guide.InputsOutputs)
	section("Keys and key management", l.Guide.KeyManagement)
	if len(l.Guide.CommonMistakes) > 0 {
		b.WriteString("## Common mistakes\n\n")
		for _, m := range l.Guide.CommonMistakes {
			fmt.Fprintf(&b, "- %s\n", m)
		}
		b.WriteString("\n")
	}
	section("Use in new systems", l.Guide.NewSystems)
	if len(l.Vectors) > 0 {
		b.WriteString("## Published test vectors\n\n")
		for _, vp := range l.Vectors {
			for _, f := range vfiles {
				if f.Path != vp {
					continue
				}
				for _, v := range f.Vectors {
					src := v.Source.Document
					if v.Source.TestCase != "" {
						src += ", " + v.Source.TestCase
					}
					fmt.Fprintf(&b, "- `%s` (%s, %s): %s\n", v.ID, v.Kind, v.Status, src)
				}
			}
		}
		b.WriteString("\n")
	}
	section("Reproduce locally", "```sh\n"+strings.TrimSpace(l.Guide.Reproduce)+"\n```")
	if l.Challenge != nil {
		b.WriteString("## Challenge\n\n")
		b.WriteString(strings.TrimSpace(l.Challenge.Prompt) + "\n\n")
		keys := make([]string, 0, len(l.Challenge.PublicInputs))
		for k := range l.Challenge.PublicInputs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "- **%s**: `%v`\n", k, l.Challenge.PublicInputs[k])
		}
		b.WriteString("\n")
		if l.Payload != nil {
			fmt.Fprintf(&b, "Encrypted solution: `%s` (sha256 `%s…`, %s). Decrypt with `p123 decrypt %s`.\n\n", l.Payload.Path, l.Payload.SHA256[:16], l.Payload.Label, l.ID)
		}
	}
	if len(l.References) > 0 {
		b.WriteString("## References\n\n")
		for _, r := range l.References {
			fmt.Fprintf(&b, "- [%s](%s)\n", r.Name, r.URL)
		}
	}
	return b.String()
}
