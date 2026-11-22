package git

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ValidateBranchName reports whether name is acceptable as a new local branch.
// It uses `git check-ref-format --branch` and rejects a name git would rewrite,
// so callers never silently store a different ref than the one the user asked for.
func ValidateBranchName(name string) error {
	if name == "" || strings.TrimSpace(name) != name {
		return fmt.Errorf("invalid git branch name %q", name)
	}
	out, err := run("", "check-ref-format", "--branch", name)
	if err != nil {
		return fmt.Errorf("invalid git branch name %q (rejected by git check-ref-format --branch)", name)
	}
	if strings.TrimSpace(out) != name {
		return fmt.Errorf("invalid git branch name %q (git would rewrite it; pass the plain branch name)", name)
	}
	return nil
}

// RefuseExistingBranch returns an error when branch already exists or is
// checked out in repo. It never deletes, resets, moves, or checks out the
// branch. A nil error means the name is free to create.
func RefuseExistingBranch(repo, branch string) error {
	checkedOutAt, checkedOut, err := branchWorktree(repo, branch)
	if err != nil {
		return err
	}
	if checkedOut {
		return fmt.Errorf("branch %q is already checked out at %s; mywt will not reuse, reset, move, or delete it. Pass --branch <name> to create a different branch", branch, checkedOutAt)
	}
	exists, err := localBranchExists(repo, branch)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("branch %q already exists in %s; mywt will not reuse, reset, move, or delete it. Pass --branch <name> to create a different branch", branch, repo)
	}
	return nil
}

func localBranchExists(repo, branch string) (bool, error) {
	cmd := exec.Command("git", "-C", repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("look up branch %q: %w", branch, err)
}

// branchWorktree returns the worktree path where branch is checked out.
func branchWorktree(repo, branch string) (string, bool, error) {
	out, err := run(repo, "worktree", "list", "--porcelain")
	if err != nil {
		return "", false, fmt.Errorf("list worktrees: %w", err)
	}
	want := "branch refs/heads/" + branch
	var current string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			current = strings.TrimPrefix(line, "worktree ")
		case line == want:
			return current, true, nil
		}
	}
	return "", false, nil
}
