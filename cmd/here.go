package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/fxricky/mywt/internal/git"
	"github.com/fxricky/mywt/internal/session"
)

var hereCmd = &cobra.Command{
	Use:   "here",
	Short: "Show the session containing the current directory",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		m := loadManager()
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: get current directory: %v\n", err)
			os.Exit(1)
		}

		s, current, ok, err := m.FindByPath(cwd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if !ok {
			fmt.Fprintln(os.Stderr, "error: current directory is not inside a managed session project")
			os.Exit(1)
		}

		display, err := sessionWithCurrentBranches(s, current)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(formatHere(display, current))
	},
}

func sessionWithCurrentBranches(s *session.Session, current session.Project) (*session.Session, error) {
	display := *s
	display.Projects = append([]session.Project(nil), s.Projects...)
	for i := range display.Projects {
		branch, err := git.CurrentBranch(display.Projects[i].Path)
		if err != nil {
			if display.Projects[i].Subfolder == current.Subfolder && display.Projects[i].Path == current.Path {
				return nil, fmt.Errorf("read branch for current project %q: %w", display.Projects[i].Subfolder, err)
			}
			display.Projects[i].Branch = "unavailable"
			continue
		}
		display.Projects[i].Branch = branch
	}
	return &display, nil
}

func formatHere(s *session.Session, current session.Project) string {
	var b strings.Builder
	label := s.ID
	if s.Name != "" {
		label += " (" + s.Name + ")"
	}

	fmt.Fprintf(&b, "Session: %s\n", label)
	fmt.Fprintf(&b, "Path: %s\n", s.Dir())
	fmt.Fprintln(&b, "Projects:")
	for _, p := range s.Projects {
		marker := " "
		if p.Subfolder == current.Subfolder && p.Path == current.Path {
			marker = "*"
		}
		fmt.Fprintf(&b, "  %s %-12s branch=%s\n", marker, p.Subfolder, p.Branch)
	}
	return b.String()
}

func init() {
	rootCmd.AddCommand(hereCmd)
}
