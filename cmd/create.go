package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/fxricky/mywt/internal/session"
)

var createName string

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new empty worktree session",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		m := loadManager()
		s, err := m.Create(session.CreateOptions{Name: createName})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(s.ID)
		if s.Name != "" {
			fmt.Fprintf(os.Stderr, "name: %s\n", s.Name)
		}
		fmt.Fprintf(os.Stderr, "path: %s\n", s.Dir())
	},
}

func init() {
	createCmd.Flags().StringVar(&createName, "name", "", "optional friendly name for the session")
	rootCmd.AddCommand(createCmd)
}
