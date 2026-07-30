package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var (
	openEditor string
	openShell  bool
)

var openCmd = &cobra.Command{
	Use:   "open <session> [project]",
	Short: "Print the path of a session or project, or open it",
	Long: `By default prints the path of a session (or a project subfolder within it),
so you can use it with cd:

  cd $(mywt open worktree-20260730123456 backend)

Use --editor <app> to open the path in an editor, or --shell to spawn a shell
inside it.`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		m := loadManager()
		s, err := m.Resolve(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		target := s.Dir()
		if len(args) == 2 {
			p, ok := s.ProjectPath(args[1])
			if !ok {
				fmt.Fprintf(os.Stderr, "error: session %s has no project %q\n", s.ID, args[1])
				os.Exit(1)
			}
			target = p
		}

		switch {
		case openShell:
			shell := os.Getenv("SHELL")
			if shell == "" {
				shell = "/bin/sh"
			}
			c := exec.Command(shell)
			c.Dir = target
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		case openEditor != "":
			c := exec.Command(openEditor, target)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		default:
			fmt.Println(target)
		}
	},
}

func init() {
	openCmd.Flags().StringVar(&openEditor, "editor", "", "open the path in this editor (e.g. code, vim)")
	openCmd.Flags().BoolVar(&openShell, "shell", false, "spawn a shell inside the path")
	rootCmd.AddCommand(openCmd)
}
