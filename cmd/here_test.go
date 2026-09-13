package cmd

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/fxricky/mywt/internal/session"
)

func TestFormatHere(t *testing.T) {
	s := &session.Session{
		ID:   "worktree-20260730123456",
		Name: "feature-x",
		Root: "/tmp/.mywt",
		Projects: []session.Project{
			{Subfolder: "backend", Branch: "feature/backend", Path: "/tmp/.mywt/worktree-20260730123456/backend"},
			{Subfolder: "frontend", Branch: "feature/frontend", Path: "/tmp/.mywt/worktree-20260730123456/frontend"},
		},
	}

	output := formatHere(s, s.Projects[1])
	for _, want := range []string{
		"Session: worktree-20260730123456 (feature-x)",
		"Path: /tmp/.mywt/worktree-20260730123456",
		"\n  * frontend",
		"branch=feature/frontend",
		"\n    backend",
		"branch=feature/backend",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("formatHere output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "/tmp/.mywt/worktree-20260730123456/backend") ||
		strings.Contains(output, "/tmp/.mywt/worktree-20260730123456/frontend") {
		t.Errorf("formatHere output should not include project paths:\n%s", output)
	}
}

func TestSessionWithCurrentBranchesKeepsUnavailableSibling(t *testing.T) {
	repo := t.TempDir()
	if output, err := exec.Command("git", "-C", repo, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	if output, err := exec.Command("git", "-C", repo, "checkout", "--quiet", "-b", "feature/current").CombinedOutput(); err != nil {
		t.Fatalf("git checkout: %v\n%s", err, output)
	}

	s := &session.Session{
		Projects: []session.Project{
			{Subfolder: "current", Path: repo, Branch: "stale/current"},
			{Subfolder: "missing", Path: repo + "-missing", Branch: "stale/missing"},
		},
	}
	display, err := sessionWithCurrentBranches(s, s.Projects[0])
	if err != nil {
		t.Fatalf("sessionWithCurrentBranches: %v", err)
	}
	if got := display.Projects[0].Branch; got != "feature/current" {
		t.Errorf("current branch = %q, want feature/current", got)
	}
	if got := display.Projects[1].Branch; got != "unavailable" {
		t.Errorf("missing sibling branch = %q, want unavailable", got)
	}
}
