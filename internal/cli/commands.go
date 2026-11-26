package cli

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"filippo.io/age"
	"github.com/Szmelc-INC/Password123/internal/config"
	"github.com/Szmelc-INC/Password123/internal/manifest"
	"github.com/Szmelc-INC/Password123/internal/pages"
	"github.com/Szmelc-INC/Password123/internal/render"
	"github.com/Szmelc-INC/Password123/internal/ui"
	"github.com/Szmelc-INC/Password123/internal/validate"
	"github.com/Szmelc-INC/Password123/internal/vault"
	"github.com/Szmelc-INC/Password123/internal/vectors"
	"github.com/charmbracelet/glamour"
	"github.com/spf13/cobra"
)

func lessonsCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "lessons",
		Aliases: []string{"lesson", "ls"},
		Short:   "List and read public lessons",
		Example: "p123 lessons list\np123 lessons list --module 02-stream-and-block --status stable\np123 lessons show 02-aes-256-gcm",
	}
	var module, status string
	list := &cobra.Command{
		Use:   "list",
		Short: "List lessons as an aligned table",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := a.repo()
			if err != nil {
				return err
			}
			lessons, err := manifest.LoadLessons(root)
			if err != nil {
				return err
			}
			var sel []manifest.Lesson
			for _, l := range lessons {
				if (module == "" || strings.HasPrefix(l.Module, module)) && (status == "" || l.Status == status) {
					sel = append(sel, l)
				}
			}
			if a.jsonOut {
				return a.printJSON(sel)
			}
			t := &ui.Table{Headers: []string{"ID", "TITLE", "STATUS", "LEVEL", "VECTORS", "CHALLENGE"}}
			for _, l := range sel {
				ch := ui.Muted.Render("-")
				if l.Payload != nil {
					ch = ui.Cmd.Render(l.Payload.Label)
				}
				t.Add(ui.Cmd.Render(l.ID), l.Title, ui.Status(l.Status), l.Difficulty, strconv.Itoa(len(l.Vectors)), ch)
			}
			t.Fprint(a.out)
			return nil
		},
	}
	list.Flags().StringVar(&module, "module", "", "filter by module prefix (e.g. 02 or 02-stream-and-block)")
	list.Flags().StringVar(&status, "status", "", "filter by status: draft, experimental, stable, deprecated")
	var raw bool
	var width int
	show := &cobra.Command{
		Use:               "show <lesson-id>",
		Short:             "Render one lesson as Markdown",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.completeLessons,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := a.repo()
			if err != nil {
				return err
			}
			lessons, err := manifest.LoadLessons(root)
			if err != nil {
				return err
			}
			l, ok := manifest.Find(lessons, args[0])
			if !ok {
				return fmt.Errorf("unknown lesson %q (see p123 lessons list)", args[0])
			}
			if a.jsonOut {
				return a.printJSON(l)
			}
			vf, _ := vectors.LoadAll(root)
			md := render.LessonMarkdown(*l, vf)
			if raw || !ui.ColorEnabled() {
				fmt.Fprint(a.out, md)
				return nil
			}
			r, err := glamour.NewTermRenderer(glamour.WithStylesFromJSONBytes(GlamourStyle()), glamour.WithWordWrap(width))
			if err != nil {
				return err
			}
			s, err := r.Render(md)
			if err != nil {
				return err
			}
			fmt.Fprint(a.out, s)
			return nil
		},
	}
	show.Flags().BoolVar(&raw, "raw", false, "print Markdown source instead of rendering it")
	show.Flags().IntVar(&width, "width", 100, "wrap width for rendered output")
	cmd.AddCommand(list, show)
	return cmd
}

func (a *app) completeLessons(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	root, err := a.repo()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	lessons, err := manifest.LoadLessons(root)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var out []string
	for _, l := range lessons {
		if strings.HasPrefix(l.ID, toComplete) {
			out = append(out, l.ID+"\t"+l.Title)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

func vectorsCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "vectors",
		Short:   "List and verify published test vectors",
		Example: "p123 vectors verify\np123 vectors verify --lesson 03-x25519\np123 vectors list --json",
	}
	var lesson string
	load := func() ([]*vectors.File, error) {
		root, err := a.repo()
		if err != nil {
			return nil, err
		}
		files, err := vectors.LoadAll(root)
		if err != nil {
			return nil, err
		}
		if lesson == "" {
			return files, nil
		}
		var sel []*vectors.File
		for _, f := range files {
			if f.LessonID == lesson {
				sel = append(sel, f)
			}
		}
		if len(sel) == 0 {
			return nil, fmt.Errorf("no vectors for lesson %q", lesson)
		}
		return sel, nil
	}
	list := &cobra.Command{
		Use:   "list",
		Short: "List vectors and their sources",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			files, err := load()
			if err != nil {
				return err
			}
			if a.jsonOut {
				return a.printJSON(files)
			}
			t := &ui.Table{Headers: []string{"LESSON", "VECTOR", "KIND", "STATUS", "SOURCE"}}
			for _, f := range files {
				for _, v := range f.Vectors {
					t.Add(ui.Cmd.Render(f.LessonID), v.ID, v.Kind, ui.Status(v.Status), v.Source.Document)
				}
			}
			t.Fprint(a.out)
			return nil
		},
	}
	verify := &cobra.Command{
		Use:   "verify",
		Short: "Recompute every vector with a maintained implementation",
		Long:  "Recomputes every vector. A vector labelled verified that fails is an error and exits with status 1.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			files, err := load()
			if err != nil {
				return err
			}
			res := vectors.VerifyAll(files)
			failed := 0
			for _, r := range res {
				if !r.OK && r.Status == "verified" {
					failed++
				}
			}
			if a.jsonOut {
				if err := a.printJSON(res); err != nil {
					return err
				}
			} else {
				t := &ui.Table{Headers: []string{"LESSON", "VECTOR", "KIND", "RESULT", "DETAIL"}}
				for _, r := range res {
					result, detail := ui.Status("pass"), ""
					if !r.OK {
						result, detail = ui.Status("fail"), r.Error
					}
					t.Add(ui.Cmd.Render(r.LessonID), r.ID, r.Kind, result, detail)
				}
				t.Fprint(a.out)
				fmt.Fprintln(a.out)
				if failed == 0 {
					fmt.Fprintln(a.out, ui.Highlight.Render(fmt.Sprintf("%d/%d vectors verified", len(res), len(res))))
				} else {
					fmt.Fprintln(a.out, ui.DangerBox.Render(fmt.Sprintf("%d verified vectors fail", failed)))
				}
			}
			if failed > 0 {
				return errSilent
			}
			return nil
		},
	}
	for _, c := range []*cobra.Command{list, verify} {
		c.Flags().StringVar(&lesson, "lesson", "", "only this lesson ID")
		c.RegisterFlagCompletionFunc("lesson", a.completeLessons)
	}
	cmd.AddCommand(list, verify)
	return cmd
}

func validateCmd(a *app) *cobra.Command {
	var skipVectors bool
	var site string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Run every fail-closed repository check",
		Long: `Runs the checks a commit must pass: schemas, unique IDs, references,
payload headers and digests, demo-identity isolation, SOPS encryption,
private key scanning, forbidden plaintext paths, portal CSP, and vectors.`,
		Example: "p123 validate\np123 validate --site _site\np123 validate --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := a.repo()
			if err != nil {
				return err
			}
			var extra []string
			if site != "" {
				extra = append(extra, site)
			}
			fs := validate.Run(root, validate.Options{SkipVectors: skipVectors, ExtraScanDirs: extra})
			if a.jsonOut {
				if err := a.printJSON(fs); err != nil {
					return err
				}
			} else {
				t := &ui.Table{Headers: []string{"CHECK", "LEVEL", "PATH", "MESSAGE"}}
				errs, warns := 0, 0
				for _, f := range fs {
					switch f.Level {
					case validate.Error:
						errs++
					case validate.Warn:
						warns++
					}
					t.Add(ui.Cmd.Render(f.Check), ui.Status(string(f.Level)), f.Path, f.Message)
				}
				t.Fprint(a.out)
				fmt.Fprintln(a.out)
				if errs == 0 {
					fmt.Fprintln(a.out, ui.Highlight.Render("validation passed"), ui.Muted.Render(fmt.Sprintf("%d warnings", warns)))
				} else {
					fmt.Fprintln(a.out, ui.DangerBox.Render(fmt.Sprintf("validation failed: %d errors", errs)), ui.Muted.Render(fmt.Sprintf("%d warnings", warns)))
				}
			}
			if validate.Failed(fs) {
				return errSilent
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&skipVectors, "skip-vectors", false, "skip vector recomputation (faster pre-commit runs)")
	cmd.Flags().StringVar(&site, "site", "", "also scan a built site directory for private key material")
	return cmd
}

func (a *app) recipientsFile(root, flag string) string {
	switch {
	case flag != "":
		return flag
	case a.cfg != nil && a.cfg.Recipients != "":
		return a.cfg.Recipients
	}
	return filepath.Join(root, "recipients", "maintainers.txt")
}

func encryptCmd(a *app) *cobra.Command {
	var lesson, demo, out, recipients string
	var armor, noSchema bool
	cmd := &cobra.Command{
		Use:   "encrypt <plaintext-file>",
		Short: "Encrypt a challenge payload to the enrolled recipients",
		Long: `Encrypts a plaintext file that lives OUTSIDE the repository.

With --lesson the ciphertext replaces that lesson's payload and the manifest
digest is updated. With --demo the file is encrypted only to the public demo
recipient and must contain demo material. JSON payloads are validated against
schemas/payload.schema.json before encryption.`,
		Example: "umask 077\np123 encrypt ~/private-work/02-aes-256-gcm.json --lesson 02-aes-256-gcm\np123 encrypt notes.json --out /tmp/notes.age -R other-recipients.txt",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := a.repo()
			if err != nil {
				return err
			}
			src := args[0]
			if vault.Inside(root, src) {
				return vault.ErrPlaintextInRepo
			}
			pt, err := os.ReadFile(src)
			if err != nil {
				return err
			}
			if n := b2i(lesson != "") + b2i(demo != "") + b2i(out != ""); n != 1 {
				return errors.New("choose exactly one of --lesson, --demo, --out")
			}
			var recips []age.Recipient
			var target string
			var lp *manifest.Lesson
			isJSON := bytes.HasPrefix(bytes.TrimSpace(pt), []byte("{"))
			switch {
			case demo != "":
				d, err := manifest.LoadDemo(root)
				if err != nil {
					return err
				}
				for _, p := range d.Payloads {
					if p.ID == demo {
						target = p.Path
					}
				}
				if target == "" {
					return fmt.Errorf("demo payload %q is not listed in demo/manifest.yaml", demo)
				}
				r, _, err := vault.ParseRecipient(d.Recipient)
				if err != nil {
					return err
				}
				recips = []age.Recipient{r}
			default:
				entries, err := vault.LoadRecipients(a.recipientsFile(root, recipients))
				if err != nil {
					return err
				}
				recips = vault.Recipients(entries)
				if lesson != "" {
					lessons, err := manifest.LoadLessons(root)
					if err != nil {
						return err
					}
					l, ok := manifest.Find(lessons, lesson)
					if !ok || l.Payload == nil {
						return fmt.Errorf("lesson %q has no payload entry in its manifest", lesson)
					}
					lp, target = l, l.Payload.Path
					if l.Payload.Format == "application/json" {
						isJSON = true
					}
				} else {
					target = out
				}
			}
			if isJSON && !noSchema {
				if err := validate.ValidatePayloadJSON(root, pt); err != nil {
					return err
				}
			}
			ct, err := vault.Encrypt(pt, recips, armor || (a.cfg != nil && a.cfg.Armor))
			if err != nil {
				return err
			}
			dst := target
			if !filepath.IsAbs(dst) && (lesson != "" || demo != "") {
				dst = filepath.Join(root, target)
			}
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(dst, ct, 0o644); err != nil {
				return err
			}
			digest := vault.SHA256Hex(ct)
			switch {
			case lp != nil:
				if err := manifest.SetPayloadDigest(root, lp, digest); err != nil {
					return err
				}
			case demo != "":
				if err := manifest.SetDemoDigest(root, demo, digest, vault.SHA256Hex(pt)); err != nil {
					return err
				}
			}
			if a.jsonOut {
				return a.printJSON(map[string]any{"path": target, "sha256": digest, "recipients": len(recips)})
			}
			fmt.Fprintf(a.out, "%s %s %s\n", ui.Badge.Render("encrypted"), target, ui.Muted.Render(fmt.Sprintf("sha256 %s, %d recipients", digest, len(recips))))
			fmt.Fprintln(a.out, ui.Muted.Render("Delete the plaintext source when you no longer need it."))
			return nil
		},
	}
	cmd.Flags().StringVarP(&lesson, "lesson", "l", "", "write the lesson's payload and update its manifest digest")
	cmd.Flags().StringVar(&demo, "demo", "", "write a demo-only payload from demo/manifest.yaml")
	cmd.Flags().StringVarP(&out, "out", "o", "", "write ciphertext to this path instead")
	cmd.Flags().StringVarP(&recipients, "recipients", "R", "", "recipients file (default: recipients/maintainers.txt)")
	cmd.Flags().BoolVarP(&armor, "armor", "a", false, "write ASCII-armored output")
	cmd.Flags().BoolVar(&noSchema, "no-schema", false, "skip payload JSON schema validation")
	cmd.RegisterFlagCompletionFunc("lesson", a.completeLessons)
	return cmd
}

// b2i converts a bool to 0 or 1 for flag exclusivity checks.
func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func decryptCmd(a *app) *cobra.Command {
	var identity, out string
	cmd := &cobra.Command{
		Use:   "decrypt <lesson-id | demo-id | payload.age>",
		Short: "Decrypt a payload locally with your private identity",
		Long: `Decrypts a payload with an identity that stays on this machine. Output goes
to stdout by default. Writing decrypted output inside the repository is refused.`,
		Example:           "p123 decrypt 02-aes-256-gcm\np123 decrypt demo-welcome -i demo/demo-identity.txt\np123 decrypt payloads/01-caesar.age -o /tmp/01-caesar.json",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.completeLessons,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, _ := a.repo()
			path, err := resolvePayload(root, args[0])
			if err != nil {
				return err
			}
			ct, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			idPath := a.identityPath(identity)
			if root != "" && vault.Inside(root, idPath) && filepath.ToSlash(mustRel(root, idPath)) != validate.DemoIdentityPath {
				return errors.New("refusing to use an identity stored inside the repository")
			}
			ids, err := vault.LoadIdentities(idPath)
			if err != nil {
				return fmt.Errorf("identity %s: %w", idPath, err)
			}
			pt, err := vault.Decrypt(ct, ids)
			if err != nil {
				var nm *age.NoIdentityMatchError
				if errors.As(err, &nm) {
					return fmt.Errorf("%s is not encrypted to identity %s", args[0], idPath)
				}
				return err
			}
			if out == "" || out == "-" {
				_, err := a.out.Write(pt)
				return err
			}
			if err := vault.WriteSecret(root, out, pt); err != nil {
				return err
			}
			fmt.Fprintf(a.errOut, "%s %s (mode 0600). Delete it when finished.\n", ui.Badge.Render("decrypted"), out)
			return nil
		},
	}
	cmd.Flags().StringVarP(&identity, "identity", "i", "", "age identity file (default: config identity or "+config.DefaultIdentity()+")")
	cmd.Flags().StringVarP(&out, "out", "o", "", "write plaintext to this file (outside the repository) instead of stdout")
	return cmd
}

func mustRel(root, p string) string {
	abs, _ := filepath.Abs(p)
	r, err := filepath.Rel(root, abs)
	if err != nil {
		return p
	}
	return r
}

func resolvePayload(root, arg string) (string, error) {
	if strings.HasSuffix(arg, ".age") {
		if _, err := os.Stat(arg); err == nil {
			return arg, nil
		}
		if root != "" {
			return filepath.Join(root, arg), nil
		}
		return arg, nil
	}
	if root == "" {
		return "", errors.New("lesson IDs need a repository checkout; pass a payload path instead")
	}
	lessons, err := manifest.LoadLessons(root)
	if err != nil {
		return "", err
	}
	if l, ok := manifest.Find(lessons, arg); ok {
		if l.Payload == nil {
			return "", fmt.Errorf("lesson %s has no encrypted payload", arg)
		}
		return filepath.Join(root, l.Payload.Path), nil
	}
	if d, err := manifest.LoadDemo(root); err == nil {
		for _, p := range d.Payloads {
			if p.ID == arg {
				return filepath.Join(root, p.Path), nil
			}
		}
	}
	return "", fmt.Errorf("unknown lesson or demo payload %q", arg)
}

func rotateCmd(a *app) *cobra.Command {
	var identity, recipients, lesson string
	var all, armor bool
	cmd := &cobra.Command{
		Use:   "rotate",
		Short: "Re-encrypt payloads to the current recipient set",
		Long: `Decrypts each payload in memory with an authorized identity and encrypts it
again to the current recipients file, then updates manifest digests. Run it
after enrolling or removing a recipient. Plaintext is never written to disk.

Rotation protects future versions only: a removed recipient may keep copies
of old ciphertext and plaintext.`,
		Example: "p123 rotate --all\np123 rotate --lesson 02-aes-256-gcm -i ~/.config/age/password123.txt",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if all == (lesson != "") {
				return errors.New("choose --all or --lesson")
			}
			root, err := a.repo()
			if err != nil {
				return err
			}
			ids, err := vault.LoadIdentities(a.identityPath(identity))
			if err != nil {
				return err
			}
			entries, err := vault.LoadRecipients(a.recipientsFile(root, recipients))
			if err != nil {
				return err
			}
			lessons, err := manifest.LoadLessons(root)
			if err != nil {
				return err
			}
			type row struct {
				ID, Path, SHA256 string
			}
			var done []row
			for i := range lessons {
				l := &lessons[i]
				if l.Payload == nil || (lesson != "" && l.ID != lesson) {
					continue
				}
				p := filepath.Join(root, l.Payload.Path)
				ct, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				pt, err := vault.Decrypt(ct, ids)
				if err != nil {
					return fmt.Errorf("%s: %w (is your identity an enrolled recipient?)", l.ID, err)
				}
				nct, err := vault.Encrypt(pt, vault.Recipients(entries), armor || (a.cfg != nil && a.cfg.Armor))
				for j := range pt {
					pt[j] = 0
				}
				if err != nil {
					return err
				}
				if err := os.WriteFile(p, nct, 0o644); err != nil {
					return err
				}
				d := vault.SHA256Hex(nct)
				if err := manifest.SetPayloadDigest(root, l, d); err != nil {
					return err
				}
				done = append(done, row{l.ID, l.Payload.Path, d})
			}
			if len(done) == 0 {
				return errors.New("no payloads matched")
			}
			if a.jsonOut {
				return a.printJSON(done)
			}
			t := &ui.Table{Headers: []string{"LESSON", "PAYLOAD", "NEW SHA256"}}
			for _, r := range done {
				t.Add(ui.Cmd.Render(r.ID), r.Path, r.SHA256)
			}
			t.Fprint(a.out)
			fmt.Fprintf(a.out, "\n%s %s\n", ui.Badge.Render(fmt.Sprintf("rotated %d payloads", len(done))), ui.Muted.Render(fmt.Sprintf("to %d recipients; test decryption with every remaining identity", len(entries))))
			return nil
		},
	}
	cmd.Flags().StringVarP(&identity, "identity", "i", "", "authorized age identity used to decrypt")
	cmd.Flags().StringVarP(&recipients, "recipients", "R", "", "recipients file (default: recipients/maintainers.txt)")
	cmd.Flags().StringVarP(&lesson, "lesson", "l", "", "rotate one lesson")
	cmd.Flags().BoolVar(&all, "all", false, "rotate every lesson payload")
	cmd.Flags().BoolVarP(&armor, "armor", "a", false, "write ASCII-armored output")
	cmd.RegisterFlagCompletionFunc("lesson", a.completeLessons)
	return cmd
}

func recipientsCmd(a *app) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:     "recipients",
		Short:   "List, check and enroll public recipients",
		Example: "p123 recipients list\np123 recipients add age1... --comment \"alice laptop 2026-11\"",
	}
	list := &cobra.Command{
		Use:   "list",
		Short: "List enrolled recipients",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := a.repo()
			if err != nil {
				return err
			}
			entries, err := vault.LoadRecipients(a.recipientsFile(root, file))
			if err != nil {
				return err
			}
			if a.jsonOut {
				type j struct {
					Line                     int
					Kind, Recipient, Comment string
				}
				var out []j
				for _, e := range entries {
					out = append(out, j{e.Line, e.Kind, e.Value, e.Comment})
				}
				return a.printJSON(out)
			}
			t := &ui.Table{Headers: []string{"LINE", "KIND", "RECIPIENT", "COMMENT"}}
			for _, e := range entries {
				v := e.Value
				if len(v) > 40 {
					v = v[:20] + "…" + v[len(v)-12:]
				}
				t.Add(strconv.Itoa(e.Line), e.Kind, ui.Cmd.Render(v), e.Comment)
			}
			t.Fprint(a.out)
			return nil
		},
	}
	var comment string
	add := &cobra.Command{
		Use:   "add <recipient>",
		Short: "Enroll a public recipient (then run p123 rotate --all)",
		Long: `Validates and appends a public recipient. Confirm the key with its owner over
a second channel first. Existing payloads are not readable by the new
recipient until you run 'p123 rotate --all'.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := a.repo()
			if err != nil {
				return err
			}
			rec := strings.TrimSpace(args[0])
			for _, p := range vault.PrivatePatterns {
				if p.MatchString(rec) {
					return errors.New("that is a PRIVATE key; share and enroll only the public recipient")
				}
			}
			_, kind, err := vault.ParseRecipient(rec)
			if err != nil {
				return err
			}
			path := a.recipientsFile(root, file)
			if entries, err := vault.LoadRecipients(path); err == nil {
				for _, e := range entries {
					if e.Value == rec {
						return errors.New("recipient is already enrolled")
					}
				}
			}
			if d, err := manifest.LoadDemo(root); err == nil && d.Recipient == rec {
				return errors.New("the demo recipient must never be enrolled for real payloads")
			}
			line := rec
			if comment != "" {
				line += " # " + strings.ReplaceAll(comment, "\n", " ")
			}
			f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := fmt.Fprintln(f, line); err != nil {
				return err
			}
			fmt.Fprintf(a.out, "%s %s recipient added to %s\n", ui.Badge.Render("enrolled"), kind, mustRel(root, path))
			fmt.Fprintln(a.out, ui.Muted.Render("Next: p123 rotate --all, then test decryption with the new identity."))
			return nil
		},
	}
	add.Flags().StringVar(&comment, "comment", "", "owner/device note stored next to the key (public!)")
	cmd.PersistentFlags().StringVarP(&file, "file", "f", "", "recipients file (default: recipients/maintainers.txt)")
	cmd.AddCommand(list, add)
	return cmd
}

func pagesCmd(a *app) *cobra.Command {
	var outDir, addr string
	var noBuild bool
	cmd := &cobra.Command{
		Use:     "pages",
		Short:   "Build, serve and verify the static browser portal",
		Example: "p123 pages build\np123 pages serve --addr 127.0.0.1:8123\np123 pages verify --dir ./downloaded-site",
	}
	build := &cobra.Command{
		Use:   "build",
		Short: "Build the portal into _site (public data and ciphertext only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := a.repo()
			if err != nil {
				return err
			}
			if fs := validate.Run(root, validate.Options{SkipVectors: true}); validate.Failed(fs) {
				for _, f := range fs {
					if f.Level == validate.Error {
						fmt.Fprintln(a.errOut, ui.ErrStyle.Render("error"), f.Check, f.Path, f.Message)
					}
				}
				return errors.New("repository validation failed; refusing to build")
			}
			dir := outDir
			if dir == "" {
				dir = filepath.Join(root, "_site")
			}
			files, err := pages.Build(root, pages.Options{OutDir: dir, Version: Version})
			if err != nil {
				return err
			}
			if fs := validate.Run(root, validate.Options{SkipVectors: true, ExtraScanDirs: []string{dir}}); validate.Failed(fs) {
				return errors.New("built site failed the private-material scan")
			}
			if a.jsonOut {
				return a.printJSON(map[string]any{"dir": dir, "files": files})
			}
			fmt.Fprintf(a.out, "%s %s %s\n", ui.Badge.Render("built"), dir, ui.Muted.Render(fmt.Sprintf("%d files + SHA256SUMS", len(files))))
			return nil
		},
	}
	serve := &cobra.Command{
		Use:   "serve",
		Short: "Serve the built portal locally with the portal CSP header",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := a.repo()
			if err != nil {
				return err
			}
			dir := outDir
			if dir == "" {
				dir = filepath.Join(root, "_site")
			}
			if !noBuild {
				if _, err := pages.Build(root, pages.Options{OutDir: dir, Version: Version}); err != nil {
					return err
				}
			}
			host, _, _ := net.SplitHostPort(addr)
			if host != "127.0.0.1" && host != "localhost" && host != "::1" {
				fmt.Fprintln(a.errOut, ui.WarnStyle.Render("warning:"), "serving on a non-loopback address")
			}
			fmt.Fprintf(a.out, "%s http://%s/\n", ui.Badge.Render("serving"), addr)
			srv := &http.Server{Addr: addr, Handler: pages.Handler(dir), ReadHeaderTimeout: 5 * time.Second}
			return srv.ListenAndServe()
		},
	}
	verify := &cobra.Command{
		Use:   "verify",
		Short: "Check a downloaded site against its SHA256SUMS",
		Long: `Recomputes the SHA-256 of every file listed in SHA256SUMS. Compare the
SHA256SUMS digest itself with the signed release attestation (see
ARCHITECTURE.md, Phase 4) before trusting the portal with a private identity.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := outDir
			if dir == "" {
				root, err := a.repo()
				if err != nil {
					return err
				}
				dir = filepath.Join(root, "_site")
			}
			sums, err := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
			if err != nil {
				return err
			}
			bad, n := 0, 0
			for _, line := range strings.Split(strings.TrimSpace(string(sums)), "\n") {
				want, name, ok := strings.Cut(line, "  ")
				if !ok || strings.Contains(name, "..") {
					return fmt.Errorf("malformed SHA256SUMS line %q", line)
				}
				n++
				b, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil || vault.SHA256Hex(b) != want {
					bad++
					fmt.Fprintln(a.errOut, ui.ErrStyle.Render("mismatch"), name)
				}
			}
			if bad > 0 {
				return fmt.Errorf("%d of %d files do not match SHA256SUMS", bad, n)
			}
			fmt.Fprintf(a.out, "%s %d files match; SHA256SUMS sha256 %s\n", ui.Badge.Render("verified"), n, vault.SHA256Hex(sums))
			return nil
		},
	}
	for _, c := range []*cobra.Command{build, serve, verify} {
		c.Flags().StringVar(&outDir, "dir", "", "site directory (default: <repo>/_site)")
	}
	serve.Flags().StringVar(&addr, "addr", "127.0.0.1:8123", "listen address")
	serve.Flags().BoolVar(&noBuild, "no-build", false, "serve the existing directory without rebuilding")
	cmd.AddCommand(build, serve, verify)
	return cmd
}

func configCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show or create the p123 config file",
	}
	path := func() string {
		if a.configFlag != "" {
			return a.configFlag
		}
		if p := os.Getenv("P123_CONFIG"); p != "" {
			return p
		}
		return config.DefaultPath()
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "path",
			Short: "Print the config file path",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				fmt.Fprintln(a.out, path())
				return nil
			},
		},
		&cobra.Command{
			Use:   "show",
			Short: "Print the effective configuration",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				root, _ := a.repo()
				eff := map[string]any{
					"config_file": path(),
					"repo":        root,
					"identity":    a.identityPath(""),
					"recipients":  a.recipientsFile(root, ""),
					"color":       a.cfg.Color,
					"armor":       a.cfg.Armor,
				}
				if a.jsonOut {
					return a.printJSON(eff)
				}
				t := &ui.Table{Headers: []string{"KEY", "VALUE"}}
				for _, k := range []string{"config_file", "repo", "identity", "recipients", "color", "armor"} {
					t.Add(ui.Cmd.Render(k), fmt.Sprint(eff[k]))
				}
				t.Fprint(a.out)
				return nil
			},
		},
		&cobra.Command{
			Use:   "init",
			Short: "Write a commented config template (never overwrites)",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				p := path()
				if _, err := os.Stat(p); err == nil {
					return fmt.Errorf("%s already exists", p)
				}
				if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
					return err
				}
				if err := os.WriteFile(p, []byte(config.Template), 0o600); err != nil {
					return err
				}
				fmt.Fprintln(a.out, ui.Badge.Render("created"), p)
				return nil
			},
		},
	)
	return cmd
}
