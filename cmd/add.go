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
basename (override with --as). A new branch is created in that repo.

Must be run from inside a git repository.

Examples:
  mywt add worktree-20260730123456
  mywt add worktree-20260730123456 --as be --base develop
  mywt add my-session --branch feature-x`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		m := loadManager()
		s, err := m.Resolve(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		s, err = m.Add(s, session.AddOptions{
			Subfolder:     addAs,
			Branch:        addBranch,
			BranchPrefix:  addBranchPrefix,
			Base:          addBase,
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
	addCmd.Flags().StringVar(&addBranch, "branch", "", "new branch name (default: <prefix>-<session-timestamp>)")
	addCmd.Flags().StringVar(&addBranchPrefix, "branch-prefix", "wt", "prefix for the generated branch name")
	addCmd.Flags().StringVar(&addBase, "base", "", "ref to start the branch from (default: repo's default branch)")
	rootCmd.AddCommand(addCmd)
}
