package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/fxricky/mywt/internal/session"
)

var listJSON bool

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all worktree sessions and their projects",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		m := loadManager()
		sessions, err := m.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if listJSON {
			if sessions == nil {
				sessions = []*session.Session{}
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(sessions)
			return
		}
		if len(sessions) == 0 {
			fmt.Fprintln(os.Stderr, "no sessions yet (run `mywt create` to start one)")
			return
		}
		for _, s := range sessions {
			label := s.ID
			if s.Name != "" {
				label = s.ID + " (" + s.Name + ")"
			}
			fmt.Printf("%s  created %s\n", label, s.CreatedAt.Local().Format("2006-01-02 15:04:05"))
			fmt.Printf("  %s\n", s.Dir())
			if len(s.Projects) == 0 {
				fmt.Println("  (no projects yet)")
			}
			for _, p := range s.Projects {
				fmt.Printf("  - %-12s branch=%s  %s\n", p.Subfolder, p.Branch, p.Path)
			}
			fmt.Println()
		}
	},
}

func init() {
	listCmd.Flags().BoolVar(&listJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(listCmd)
}
