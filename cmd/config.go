package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/fxricky/mywt/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show mywt configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the worktree root where sessions are stored",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		root, err := config.Root()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("root = %s\n", root)
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
	rootCmd.AddCommand(configCmd)
}
