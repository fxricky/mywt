package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/fxricky/mywt/internal/session"
)

var (
	removeForce          bool
	removeDeleteBranches bool
)

var removeCmd = &cobra.Command{
	Use:   "remove <session>",
	Short: "Remove a session and all its project worktrees",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		m := loadManager()
		s, err := m.Resolve(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if err := m.Remove(s, session.RemoveOptions{
			Force:          removeForce,
			DeleteBranches: removeDeleteBranches,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "removed session %s\n", s.ID)
	},
}

func init() {
	removeCmd.Flags().BoolVar(&removeForce, "force", false, "force-remove worktrees with modifications")
	removeCmd.Flags().BoolVar(&removeDeleteBranches, "delete-branches", false, "also delete the branches created for each project")
	rootCmd.AddCommand(removeCmd)
}
