// Package tui is the full-screen terminal browser for Password123: a lesson
// list, a rendered Markdown pane, a ':' command palette, a '?' help overlay,
// and local payload decryption with an explicit lock action.
package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/Szmelc-INC/Password123/internal/manifest"
	"github.com/Szmelc-INC/Password123/internal/render"
	"github.com/Szmelc-INC/Password123/internal/ui"
	"github.com/Szmelc-INC/Password123/internal/validate"
	"github.com/Szmelc-INC/Password123/internal/vault"
	"github.com/Szmelc-INC/Password123/internal/vectors"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// Options configure the TUI.
type Options struct {
	Root     string
	Identity string
	Glamour  []byte
}

type overlay int

const (
	ovNone overlay = iota
	ovHelp
	ovPalette
)

type focus int

const (
	focusList focus = iota
	focusDoc
)

// command is one palette entry.
type command struct {
	name, args, help string
}

var commands = []command{
	{"verify", "[lesson-id]", "recompute vectors (current lesson by default, 'all' for every lesson)"},
	{"validate", "", "run every repository check"},
	{"decrypt", "[lesson-id]", "decrypt a payload in memory with the configured identity"},
	{"identity", "<path>", "use another identity file (memory only)"},
	{"lock", "", "drop decrypted content and the loaded identity"},
	{"module", "<prefix>", "show lessons of one module (empty for all)"},
	{"status", "<status>", "show lessons with one status (empty for all)"},
	{"open", "<lesson-id>", "jump to a lesson"},
	{"help", "", "show keybindings"},
	{"quit", "", "exit"},
}

type keymap struct {
	Up, Down, Tab, Enter, Help, Palette, Verify, Decrypt, Validate, Lock, Quit, Top, Bottom key.Binding
}

var keys = keymap{
	Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Tab:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch pane")),
	Enter:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open lesson")),
	Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Palette:  key.NewBinding(key.WithKeys(":", "ctrl+p"), key.WithHelp(":", "command palette")),
	Verify:   key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "verify vectors")),
	Decrypt:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "decrypt payload")),
	Validate: key.NewBinding(key.WithKeys("V"), key.WithHelp("V", "validate repo")),
	Lock:     key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "lock (clear plaintext)")),
	Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Top:      key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
	Bottom:   key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")),
}

type item struct{ l manifest.Lesson }

func (i item) Title() string { return i.l.ID }
func (i item) Description() string {
	s := i.l.Title + " · " + i.l.Status
	if i.l.Payload != nil {
		s += " · challenge"
	}
	return s
}
func (i item) FilterValue() string { return i.l.ID + " " + i.l.Title + " " + i.l.Category }

type model struct {
	opt      Options
	lessons  []manifest.Lesson
	vfiles   []*vectors.File
	list     list.Model
	doc      viewport.Model
	input    textinput.Model
	renderer *glamour.TermRenderer
	overlay  overlay
	focus    focus
	width    int
	height   int
	status   string
	statusOK bool
	current  string
	// extra is appended below the lesson (vector results, plaintext).
	extra     string
	plaintext bool
	identity  string
	module    string
	stat      string
}

// Run starts the TUI.
func Run(opt Options) error {
	m, err := newModel(opt)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func newModel(opt Options) (*model, error) {
	lessons, err := manifest.LoadLessons(opt.Root)
	if err != nil {
		return nil, err
	}
	vf, err := vectors.LoadAll(opt.Root)
	if err != nil {
		return nil, err
	}
	d := list.NewDefaultDelegate()
	d.Styles.SelectedTitle = d.Styles.SelectedTitle.Foreground(ui.AccentText).BorderForeground(ui.Accent)
	d.Styles.SelectedDesc = d.Styles.SelectedDesc.Foreground(ui.Silver).BorderForeground(ui.Accent)
	d.Styles.NormalTitle = d.Styles.NormalTitle.Foreground(lipgloss.Color("#d0d0d0"))
	d.Styles.NormalDesc = d.Styles.NormalDesc.Foreground(ui.Grey)
	l := list.New(nil, d, 30, 20)
	l.Title = "Password123"
	l.Styles.Title = ui.Title
	l.SetShowHelp(false)
	l.SetStatusBarItemName("lesson", "lessons")
	l.Styles.FilterPrompt = l.Styles.FilterPrompt.Foreground(ui.AccentText)
	l.Styles.FilterCursor = l.Styles.FilterCursor.Foreground(ui.AccentText)
	in := textinput.New()
	in.Prompt = ":"
	in.PromptStyle = ui.Cmd
	in.Placeholder = "verify | validate | decrypt | lock | module 02 | help | quit"
	m := &model{opt: opt, lessons: lessons, vfiles: vf, list: l, doc: viewport.New(60, 20), input: in, identity: opt.Identity}
	m.applyFilter()
	m.status = "? help · : commands · / filter · q quit"
	return m, nil
}

func (m *model) applyFilter() {
	var items []list.Item
	for _, l := range m.lessons {
		if (m.module == "" || strings.HasPrefix(l.Module, m.module)) && (m.stat == "" || l.Status == m.stat) {
			items = append(items, item{l})
		}
	}
	m.list.SetItems(items)
}

func (m *model) Init() tea.Cmd { return nil }

func (m *model) selected() (*manifest.Lesson, bool) {
	it, ok := m.list.SelectedItem().(item)
	if !ok {
		return nil, false
	}
	return &it.l, true
}

func (m *model) layout() {
	lw := max(28, m.width/3)
	m.list.SetSize(lw, m.height-2)
	m.doc.Width = m.width - lw - 3
	m.doc.Height = m.height - 3
	m.renderer, _ = glamour.NewTermRenderer(glamour.WithStylesFromJSONBytes(m.opt.Glamour), glamour.WithWordWrap(max(20, m.doc.Width-4)))
	m.refreshDoc()
}

func (m *model) refreshDoc() {
	l, ok := m.selected()
	if !ok {
		m.doc.SetContent(ui.Muted.Render("no lessons match the current filter"))
		return
	}
	if l.ID != m.current {
		m.current = l.ID
		m.extra = ""
		m.plaintext = false
	}
	md := render.LessonMarkdown(*l, m.vfiles)
	if m.extra != "" {
		md += "\n---\n\n" + m.extra
	}
	out := md
	if m.renderer != nil {
		if s, err := m.renderer.Render(md); err == nil {
			out = s
		}
	}
	m.doc.SetContent(out)
}

func (m *model) setStatus(ok bool, format string, a ...any) {
	m.status, m.statusOK = fmt.Sprintf(format, a...), ok
}

func (m *model) verify(target string) {
	var sel []*vectors.File
	for _, f := range m.vfiles {
		if target == "all" || f.LessonID == target {
			sel = append(sel, f)
		}
	}
	if len(sel) == 0 {
		m.setStatus(false, "no vectors for %s", target)
		return
	}
	res := vectors.VerifyAll(sel)
	var b strings.Builder
	b.WriteString("## Vector verification\n\n| vector | kind | result |\n|---|---|---|\n")
	pass := 0
	for _, r := range res {
		result := "pass"
		if r.OK {
			pass++
		} else {
			result = "FAIL: " + r.Error
		}
		fmt.Fprintf(&b, "| `%s/%s` | %s | %s |\n", r.LessonID, r.ID, r.Kind, result)
	}
	m.extra = b.String()
	m.plaintext = false
	m.setStatus(pass == len(res), "%d/%d vectors pass", pass, len(res))
	m.refreshDoc()
}

func (m *model) validateRepo() {
	fs := validate.Run(m.opt.Root, validate.Options{})
	var b strings.Builder
	b.WriteString("## Repository validation\n\n| check | level | message |\n|---|---|---|\n")
	errs := 0
	for _, f := range fs {
		if f.Level == validate.Error {
			errs++
		}
		msg := strings.ReplaceAll(f.Message, "|", "/")
		if f.Path != "" {
			msg = "`" + f.Path + "` " + msg
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", f.Check, f.Level, msg)
	}
	m.extra = b.String()
	m.plaintext = false
	m.setStatus(errs == 0, "validation: %d errors", errs)
	m.refreshDoc()
}

func (m *model) decrypt(id string) {
	l, ok := manifest.Find(m.lessons, id)
	if !ok || l.Payload == nil {
		m.setStatus(false, "%s has no encrypted payload", id)
		return
	}
	ct, err := os.ReadFile(m.opt.Root + "/" + l.Payload.Path)
	if err != nil {
		m.setStatus(false, "%v", err)
		return
	}
	ids, err := vault.LoadIdentities(m.identity)
	if err != nil {
		m.setStatus(false, "identity %s: %v (use :identity <path>)", m.identity, err)
		return
	}
	pt, err := vault.Decrypt(ct, ids)
	if err != nil {
		m.setStatus(false, "cannot decrypt %s with %s", id, m.identity)
		return
	}
	// Show as a fenced block so Markdown in the payload is not interpreted.
	m.extra = "## Decrypted payload (local, in memory)\n\nPress `L` to lock and clear it.\n\n```\n" + strings.ReplaceAll(string(pt), "```", "ʼʼʼ") + "\n```\n"
	for i := range pt {
		pt[i] = 0
	}
	m.plaintext = true
	m.setStatus(true, "decrypted %s locally; L locks", id)
	m.refreshDoc()
}

func (m *model) lock() {
	m.extra = ""
	m.plaintext = false
	m.identity = m.opt.Identity
	m.setStatus(true, "locked: decrypted content cleared")
	m.refreshDoc()
}

func (m *model) run(line string) tea.Cmd {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil
	}
	name, arg := fields[0], strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
	cur := m.current
	switch name {
	case "q", "quit", "exit":
		return tea.Quit
	case "help", "h":
		m.overlay = ovHelp
	case "verify", "v":
		if arg == "" {
			arg = cur
		}
		m.verify(arg)
	case "validate":
		m.validateRepo()
	case "decrypt", "d":
		if arg == "" {
			arg = cur
		}
		m.decrypt(arg)
	case "identity":
		if arg == "" {
			m.setStatus(false, "usage: identity <path>")
			break
		}
		m.identity = arg
		m.setStatus(true, "identity set to %s for this session", arg)
	case "lock":
		m.lock()
	case "module":
		m.module = arg
		m.applyFilter()
		m.refreshDoc()
		m.setStatus(true, "module filter: %q", arg)
	case "status":
		m.stat = arg
		m.applyFilter()
		m.refreshDoc()
		m.setStatus(true, "status filter: %q", arg)
	case "open":
		for i, it := range m.list.Items() {
			if it.(item).l.ID == arg {
				m.list.Select(i)
				m.refreshDoc()
				return nil
			}
		}
		m.setStatus(false, "unknown lesson %q", arg)
	default:
		m.setStatus(false, "unknown command %q (try :help)", name)
	}
	return nil
}

func (m *model) suggestions() []command {
	q := strings.Fields(m.input.Value())
	var out []command
	for _, c := range commands {
		if len(q) == 0 || strings.HasPrefix(c.name, q[0]) {
			out = append(out, c)
		}
	}
	return out
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil
	case tea.KeyMsg:
		switch m.overlay {
		case ovHelp:
			if msg.String() == "esc" || msg.String() == "?" || msg.String() == "q" || msg.String() == "enter" {
				m.overlay = ovNone
			}
			return m, nil
		case ovPalette:
			switch msg.String() {
			case "esc", "ctrl+c":
				m.overlay = ovNone
				m.input.Reset()
				return m, nil
			case "tab":
				if s := m.suggestions(); len(s) > 0 {
					m.input.SetValue(s[0].name + " ")
					m.input.CursorEnd()
				}
				return m, nil
			case "enter":
				line := m.input.Value()
				m.input.Reset()
				m.overlay = ovNone
				return m, m.run(line)
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
		if m.list.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			m.refreshDoc()
			return m, cmd
		}
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Help):
			m.overlay = ovHelp
			return m, nil
		case key.Matches(msg, keys.Palette):
			m.overlay = ovPalette
			m.input.Focus()
			return m, textinput.Blink
		case key.Matches(msg, keys.Tab):
			if m.focus == focusList {
				m.focus = focusDoc
			} else {
				m.focus = focusList
			}
			return m, nil
		case key.Matches(msg, keys.Verify):
			m.verify(m.current)
			return m, nil
		case key.Matches(msg, keys.Decrypt):
			m.decrypt(m.current)
			return m, nil
		case key.Matches(msg, keys.Validate):
			m.validateRepo()
			return m, nil
		case key.Matches(msg, keys.Lock):
			m.lock()
			return m, nil
		case key.Matches(msg, keys.Enter):
			m.focus = focusDoc
			return m, nil
		}
		var cmd tea.Cmd
		if m.focus == focusDoc {
			switch {
			case key.Matches(msg, keys.Top):
				m.doc.GotoTop()
			case key.Matches(msg, keys.Bottom):
				m.doc.GotoBottom()
			default:
				m.doc, cmd = m.doc.Update(msg)
			}
			return m, cmd
		}
		m.list, cmd = m.list.Update(msg)
		m.refreshDoc()
		return m, cmd
	case tea.MouseMsg:
		var cmd tea.Cmd
		m.doc, cmd = m.doc.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *model) View() string {
	if m.width == 0 {
		return "loading…"
	}
	border := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ui.Line)
	active := border.BorderForeground(ui.Accent)
	ls, ds := border, border
	if m.focus == focusList {
		ls = active
	} else {
		ds = active
	}
	left := ls.Width(m.list.Width()).Height(m.height - 3).Render(m.list.View())
	right := ds.Width(m.doc.Width).Height(m.height - 3).Render(m.doc.View())
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	st := ui.Secondary
	if !m.statusOK && m.status != "" && !strings.HasPrefix(m.status, "?") {
		st = ui.ErrStyle
	}
	lockBadge := ui.Badge.Render("locked")
	if m.plaintext {
		lockBadge = ui.DangerBox.Render("PLAINTEXT SHOWN")
	}
	bar := lockBadge + " " + st.Render(truncate(m.status, max(10, m.width-30))) + "  " + ui.Muted.Render(fmt.Sprintf("%d%%", int(m.doc.ScrollPercent()*100)))
	view := body + "\n" + bar
	switch m.overlay {
	case ovHelp:
		return place(m.width, m.height, m.helpView())
	case ovPalette:
		return place(m.width, m.height, m.paletteView())
	}
	return view
}

func truncate(s string, n int) string {
	if lipgloss.Width(s) <= n {
		return s
	}
	r := []rune(s)
	if len(r) > n-1 {
		r = r[:n-1]
	}
	return string(r) + "…"
}

func place(w, h int, box string) string {
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box, lipgloss.WithWhitespaceBackground(ui.Bg0))
}

var box = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ui.Accent).Background(ui.Bg1).Padding(1, 2)

func (m *model) helpView() string {
	var b strings.Builder
	b.WriteString(ui.Title.Render("Keybindings") + "\n\n")
	for _, k := range []key.Binding{keys.Up, keys.Down, keys.Tab, keys.Enter, keys.Top, keys.Bottom, keys.Verify, keys.Decrypt, keys.Validate, keys.Lock, keys.Palette, keys.Help, keys.Quit} {
		h := k.Help()
		fmt.Fprintf(&b, "  %-8s %s\n", ui.Cmd.Render(h.Key), h.Desc)
	}
	fmt.Fprintf(&b, "  %-8s %s\n", ui.Cmd.Render("/"), "filter lessons")
	b.WriteString("\n" + ui.Heading.Render("Commands") + "\n")
	for _, c := range commands {
		fmt.Fprintf(&b, "  %s %s  %s\n", ui.Cmd.Render(":"+c.name), ui.Muted.Render(c.args), c.help)
	}
	b.WriteString("\n" + ui.Muted.Render("Identities stay in memory. Plaintext is cleared on lock. esc closes."))
	return box.Render(b.String())
}

func (m *model) paletteView() string {
	var b strings.Builder
	b.WriteString(ui.Title.Render("Command palette") + "\n\n")
	b.WriteString(m.input.View() + "\n\n")
	for i, c := range m.suggestions() {
		line := fmt.Sprintf("%-10s %-14s %s", c.name, c.args, c.help)
		if i == 0 {
			b.WriteString(ui.Highlight.Render(line) + "\n")
		} else {
			b.WriteString("  " + ui.Secondary.Render(line) + "\n")
		}
	}
	b.WriteString("\n" + ui.Muted.Render("tab complete · enter run · esc close"))
	return box.Width(min(90, m.width-4)).Render(b.String())
}
