// Command p123 is the Password123 CLI and TUI.
package main

import (
	"os"

	"github.com/Szmelc-INC/Password123/internal/cli"
)

func main() { os.Exit(cli.Execute()) }
