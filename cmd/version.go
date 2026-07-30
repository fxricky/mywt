package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/fxricky/mywt/internal/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the mywt version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.Get())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
