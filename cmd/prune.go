package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var pruneDryRun bool

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Clean up stale worktrees and broken session folders",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		m := loadManager()
		res, err := m.Prune(pruneDryRun)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		verb := "pruned"
		if pruneDryRun {
			verb = "would prune"
		}
		if len(res.PrunedRepos) == 0 && len(res.BrokenSessions) == 0 {
			fmt.Fprintln(os.Stderr, "nothing to prune")
			return
		}
		for _, r := range res.PrunedRepos {
			fmt.Fprintf(os.Stderr, "%s worktrees for repo: %s\n", verb, r)
		}
		for _, d := range res.BrokenSessions {
			fmt.Fprintf(os.Stderr, "%s broken session: %s\n", verb, d)
		}
	},
}

func init() {
	pruneCmd.Flags().BoolVar(&pruneDryRun, "dry-run", false, "report what would be pruned without changing anything")
	rootCmd.AddCommand(pruneCmd)
}
