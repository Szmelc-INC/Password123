package cli

import (
	"github.com/Szmelc-INC/Password123/internal/tui"
	"github.com/spf13/cobra"
)

func tuiCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open the interactive terminal browser",
		Long:  "Browse lessons, read rendered Markdown, verify vectors and decrypt payloads in a full-screen terminal UI. Press ? for keybindings and : for the command palette.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := a.repo()
			if err != nil {
				return err
			}
			return tui.Run(tui.Options{Root: root, Identity: a.identityPath(""), Glamour: GlamourStyle()})
		},
	}
}
