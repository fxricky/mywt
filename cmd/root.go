package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/fxricky/mywt/internal/config"
	"github.com/fxricky/mywt/internal/session"
)

var rootCmd = &cobra.Command{
	Use:   "mywt",
	Short: "mywt manages git worktrees for many projects in one place",
	Long: `mywt is a tool for managing git worktrees in a single shared folder.

Each session lives at <root>/worktree-<YYYYMMDDHHmmss>/ and can hold one or
more project worktrees (e.g. backend/ and frontend/) so you can work on several
repos together under one timestamped directory.

Typical workflow:
  mywt create                       # start an empty session (run from anywhere)
  cd ~/repos/backend && mywt add worktree-<id>   # add backend worktree
  cd ~/repos/frontend && mywt add worktree-<id>  # add frontend worktree
  cd $(mywt open worktree-<id> backend)          # jump into a project
`,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// loadManager returns a session.Manager backed by the fixed worktree root
// (~/.mywt), ensuring the root directory exists.
func loadManager() *session.Manager {
	root, err := config.Root()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error: create root %s: %v\n", root, err)
		os.Exit(1)
	}
	return session.New(root)
}
