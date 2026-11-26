// Package cli implements the p123 command line.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/Szmelc-INC/Password123/internal/config"
	"github.com/Szmelc-INC/Password123/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Version is set at build time with -ldflags "-X .../internal/cli.Version=v1.2.3".
var Version = "dev"

// app carries resolved global state for commands.
type app struct {
	repoFlag   string
	configFlag string
	jsonOut    bool
	noColor    bool
	colorMode  string
	cfg        *config.Config
	out        io.Writer
	errOut     io.Writer
}

// errSilent signals a failure whose details were already printed.
var errSilent = errors.New("")

func (a *app) repo() (string, error) {
	switch {
	case a.repoFlag != "":
		if !config.IsRepo(a.repoFlag) {
			return "", fmt.Errorf("%s is not a Password123 checkout", a.repoFlag)
		}
		return a.repoFlag, nil
	case os.Getenv("P123_REPO") != "":
		return os.Getenv("P123_REPO"), nil
	}
	if r, err := config.FindRepo("."); err == nil {
		return r, nil
	}
	if a.cfg != nil && a.cfg.Repo != "" && config.IsRepo(a.cfg.Repo) {
		return a.cfg.Repo, nil
	}
	return config.FindRepo(".")
}

func (a *app) identityPath(flag string) string {
	switch {
	case flag != "":
		return config.Expand(flag)
	case os.Getenv("P123_IDENTITY") != "":
		return config.Expand(os.Getenv("P123_IDENTITY"))
	case a.cfg != nil && a.cfg.Identity != "":
		return a.cfg.Identity
	}
	return config.DefaultIdentity()
}

func (a *app) printJSON(v any) error {
	enc := json.NewEncoder(a.out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// NewRoot builds the command tree.
func NewRoot() *cobra.Command {
	a := &app{out: os.Stdout, errOut: os.Stderr}
	root := &cobra.Command{
		Use:   "p123",
		Short: "Password123 zero-knowledge cryptography lab",
		Long: `p123 is the reference CLI for the Password123 repository.

It lists public lessons, verifies published test vectors, encrypts and
decrypts age challenge payloads, rotates recipients, validates the repository
before a commit, and builds the static GitHub Pages portal.

Private identities stay outside the repository. Plaintext never enters Git.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			a.out, a.errOut = cmd.OutOrStdout(), cmd.ErrOrStderr()
			cfgPath := a.configFlag
			if cfgPath == "" {
				cfgPath = os.Getenv("P123_CONFIG")
			}
			if cfgPath == "" {
				cfgPath = config.DefaultPath()
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("config %s: %w", cfgPath, err)
			}
			a.cfg = cfg
			mode := a.colorMode
			if mode == "" {
				mode = cfg.Color
			}
			ui.SetColor(mode, a.noColor || a.jsonOut)
			return nil
		},
	}
	pf := root.PersistentFlags()
	pf.StringVar(&a.repoFlag, "repo", "", "repository checkout (default: search upward from the current directory)")
	pf.StringVar(&a.configFlag, "config", "", "config file (default: "+config.DefaultPath()+")")
	pf.BoolVar(&a.jsonOut, "json", false, "machine-readable JSON output")
	pf.BoolVar(&a.noColor, "no-color", false, "disable colors (NO_COLOR is also honoured)")
	pf.StringVar(&a.colorMode, "color", "", "color mode: auto, always, never")

	root.AddGroup(&cobra.Group{ID: "learn", Title: "Learn"}, &cobra.Group{ID: "vault", Title: "Vault"}, &cobra.Group{ID: "repo", Title: "Repository"}, &cobra.Group{ID: "other", Title: "Other"})
	add := func(group string, cmds ...*cobra.Command) {
		for _, c := range cmds {
			c.GroupID = group
			root.AddCommand(c)
		}
	}
	add("learn", lessonsCmd(a), vectorsCmd(a), tuiCmd(a))
	add("vault", encryptCmd(a), decryptCmd(a), rotateCmd(a), recipientsCmd(a))
	add("repo", validateCmd(a), pagesCmd(a))
	add("other", configCmd(a), versionCmd(a))
	root.SetCompletionCommandGroupID("other")
	root.SetHelpCommandGroupID("other")
	root.SetHelpFunc(helpFunc)
	return root
}

// Execute runs the CLI and returns the process exit code.
func Execute() int {
	root := NewRoot()
	if err := root.Execute(); err != nil {
		if err != errSilent && err.Error() != "" {
			fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("error:"), err)
		}
		return 1
	}
	return 0
}

func versionCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the p123 version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if a.jsonOut {
				return a.printJSON(map[string]string{"version": Version})
			}
			fmt.Fprintln(a.out, "p123", Version)
			return nil
		},
	}
}

// helpFunc renders colored, per-command help.
func helpFunc(cmd *cobra.Command, _ []string) {
	// Flag parsing for --no-color happens before help is shown.
	if nc, _ := cmd.Flags().GetBool("no-color"); nc {
		ui.SetColor("never", true)
	} else if c, _ := cmd.Flags().GetString("color"); c != "" {
		ui.SetColor(c, false)
	} else {
		ui.SetColor("auto", false)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, ui.Title.Render(cmd.CommandPath()), ui.Secondary.Render(cmd.Short))
	if cmd.Long != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, cmd.Long)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, ui.Heading.Render("Usage"))
	if cmd.Runnable() {
		fmt.Fprintln(w, "  "+cmd.UseLine())
	}
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintln(w, "  "+cmd.CommandPath()+" "+ui.Cmd.Render("<command>")+" [flags]")
	}
	if cmd.Example != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, ui.Heading.Render("Examples"))
		for _, l := range strings.Split(strings.TrimRight(cmd.Example, "\n"), "\n") {
			fmt.Fprintln(w, "  "+ui.Secondary.Render(strings.TrimSpace(l)))
		}
	}
	if cmd.HasAvailableSubCommands() {
		groups := map[string][]*cobra.Command{}
		var order []string
		for _, g := range cmd.Groups() {
			order = append(order, g.ID)
		}
		order = append(order, "")
		for _, c := range cmd.Commands() {
			if c.IsAvailableCommand() || c.Name() == "help" {
				groups[c.GroupID] = append(groups[c.GroupID], c)
			}
		}
		width := 0
		for _, c := range cmd.Commands() {
			width = max(width, len(c.Name()))
		}
		for _, gid := range order {
			cs := groups[gid]
			if len(cs) == 0 {
				continue
			}
			title := "Commands"
			for _, g := range cmd.Groups() {
				if g.ID == gid {
					title = g.Title
				}
			}
			fmt.Fprintln(w)
			fmt.Fprintln(w, ui.Heading.Render(title))
			sort.Slice(cs, func(i, j int) bool { return cs[i].Name() < cs[j].Name() })
			for _, c := range cs {
				fmt.Fprintf(w, "  %s%s  %s\n", ui.Cmd.Render(c.Name()), strings.Repeat(" ", width-len(c.Name())), c.Short)
			}
		}
	}
	printFlags := func(title string, fs *pflag.FlagSet) {
		if !fs.HasAvailableFlags() {
			return
		}
		fmt.Fprintln(w)
		fmt.Fprintln(w, ui.Heading.Render(title))
		var rows [][2]string
		fs.VisitAll(func(f *pflag.Flag) {
			if f.Hidden {
				return
			}
			name := "--" + f.Name
			if f.Shorthand != "" {
				name = "-" + f.Shorthand + ", " + name
			} else {
				name = "    " + name
			}
			if t := f.Value.Type(); t != "bool" {
				name += " " + t
			}
			usage := f.Usage
			if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "[]" && f.Value.Type() != "bool" {
				usage += ui.Muted.Render(" (default " + f.DefValue + ")")
			}
			rows = append(rows, [2]string{name, usage})
		})
		width := 0
		for _, r := range rows {
			width = max(width, len(r[0]))
		}
		for _, r := range rows {
			fmt.Fprintf(w, "  %s%s  %s\n", ui.Flag.Render(r[0]), strings.Repeat(" ", width-len(r[0])), r[1])
		}
	}
	printFlags("Flags", cmd.LocalNonPersistentFlags())
	printFlags("Global flags", cmd.InheritedFlags())
	if cmd == cmd.Root() {
		printFlags("Global flags", cmd.PersistentFlags())
	}
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintln(w)
		fmt.Fprintln(w, ui.Muted.Render("Run '"+cmd.CommandPath()+" <command> --help' for details on a command."))
	}
}
