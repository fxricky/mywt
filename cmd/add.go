package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/fxricky/mywt/internal/session"
)

var (
	addAs           string
	addBranch       string
	addBranchPrefix string
	addBase         string
)

var addCmd = &cobra.Command{
	Use:   "add <session>",
	Short: "Add a worktree of the current repo to a session",
	Long: `Add a worktree of the git repo enclosing the current directory to an
existing session. The project subfolder is named after the repo's directory
basename (override with --as).

The new branch defaults to the session name when the session has one, so every
repository added to "auth-refactor" starts on branch auth-refactor. An unnamed
session still uses <prefix>-<timestamp> (prefix defaults to wt). --branch
always wins. Pass --branch-prefix to force the timestamp form even for a named
session. If that branch already exists or is checked out, mywt stops and will
not reuse, reset, move, or delete it; pass --branch with a different name.

Must be run from inside a git repository.

Examples:
  mywt add auth-refactor
  mywt add auth-refactor --as backend --base develop
  mywt add worktree-20260730123456 --branch feature-x`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		m := loadManager()
		s, err := m.Resolve(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		s, err = m.Add(s, session.AddOptions{
			Subfolder:       addAs,
			Branch:          addBranch,
			BranchPrefix:    addBranchPrefix,
			BranchPrefixSet: cmd.Flags().Changed("branch-prefix"),
			Base:            addBase,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		p := s.Projects[len(s.Projects)-1]
		fmt.Fprintf(os.Stderr, "added %s -> %s (branch %s from %s)\n", p.Subfolder, p.Path, p.Branch, p.Repo)
		fmt.Println(p.Path)
	},
}

func init() {
	addCmd.Flags().StringVar(&addAs, "as", "", "name of the project subfolder (default: repo basename)")
	addCmd.Flags().StringVar(&addBranch, "branch", "", "branch to create (default: the session name, or <prefix>-<timestamp> if the session is unnamed)")
	addCmd.Flags().StringVar(&addBranchPrefix, "branch-prefix", "wt", "prefix for a generated <prefix>-<timestamp> branch (unnamed sessions, or when set explicitly)")
	addCmd.Flags().StringVar(&addBase, "base", "", "ref to start the branch from (default: repo's default branch)")
	rootCmd.AddCommand(addCmd)
}
