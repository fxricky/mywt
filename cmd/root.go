package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/fxricky/mywt/internal/config"
	"github.com/fxricky/mywt/internal/session"
)

var noFrame bool

var rootCmd = &cobra.Command{
	Use:   "mywt",
	Short: "Manage git worktree sessions and open them in a shell",
	Long: `mywt manages git worktrees for several repositories in one session folder.

With no arguments in a terminal, mywt opens a session selector. The session you
pick starts an interactive shell (from $SHELL when that program exists, otherwise /bin/sh)
whose working directory is the session root: the folder that contains every
repository worktree, not one of those worktrees. The session root is not itself
a git repository. exit returns to the shell and directory you started from.
No shell rc changes and no tmux are required.

On a terminal the shell is wrapped in a frame naming the session, the session
root, and the directory exit returns to. A footer then reports the shell's
exit status. --no-frame, or MYWT_NO_FRAME=1, prints the plain banner instead.
mywt does not change your prompt. When the terminal supports it, the window
title is set to "mywt: <session>" and restored on exit. That is not a pinned
header.

Each session is stored under ~/.mywt/worktree-<YYYYMMDDHHmmss>/. A named
session's repositories default to a git branch of that same name. Unnamed
sessions still default to wt-<timestamp>.

Typical workflow:
  mywt create --name auth-refactor
  cd ~/repos/backend && mywt add auth-refactor
  cd ~/repos/frontend && mywt add auth-refactor
  mywt                                 # select auth-refactor; shell starts at the session root
  mywt open auth-refactor backend      # print one project's path
`,
	Example: `  mywt
  mywt create --name auth-refactor
  mywt add auth-refactor --branch feature-x`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runInteractiveSelector()
	},
}

func init() {
	rootCmd.Flags().BoolVar(&noFrame, "no-frame", false, "print the plain session banner instead of the subshell frame")
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
