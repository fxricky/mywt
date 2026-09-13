package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// RepoRoot returns the absolute path of the git repo enclosing dir (the
// working tree root) by running `git rev-parse --show-toplevel` in cwd.
func RepoRoot() (string, error) {
	out, err := run("", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not inside a git repository (run mywt add from within a repo): %w", err)
	}
	return strings.TrimSpace(out), nil
}

// DefaultBranch returns the repo's default branch. It first tries
// `git symbolic-ref refs/remotes/origin/HEAD`; if that fails it falls back to
// main, then master, then the current HEAD branch.
func DefaultBranch(repo string) (string, error) {
	if out, err := run(repo, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		if b := strings.TrimSpace(out); b != "" {
			return b, nil
		}
	}
	for _, b := range []string{"main", "master"} {
		if exists, _ := branchExists(repo, b); exists {
			return b, nil
		}
	}
	// Fall back to the current branch of the source repo.
	out, err := run(repo, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("detect default branch: %w", err)
	}
	if b := strings.TrimSpace(out); b != "" && b != "HEAD" {
		return b, nil
	}
	return "", fmt.Errorf("could not determine default branch for %s", repo)
}

// CurrentBranch returns the branch currently checked out in a worktree. Git
// returns HEAD when the worktree is detached.
func CurrentBranch(worktree string) (string, error) {
	// symbolic-ref also works for an unborn branch in a newly initialized
	// repository, where rev-parse HEAD has no commit to resolve yet.
	if out, err := run(worktree, "symbolic-ref", "--short", "HEAD"); err == nil {
		if branch := strings.TrimSpace(out); branch != "" {
			return branch, nil
		}
	}

	out, err := run(worktree, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("read current branch: %w", err)
	}
	branch := strings.TrimSpace(out)
	if branch == "" {
		return "", fmt.Errorf("read current branch: empty branch name")
	}
	return branch, nil
}

// branchExists reports whether a local branch exists in repo.
func branchExists(repo, branch string) (bool, error) {
	_, err := run(repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil, nil
}

// WorktreeAdd creates a new branch and checks it out into path as a worktree.
// repo is the source repo path, branch is the new branch name, path is the
// destination worktree path, base is the commit/ref to start from (optional,
// may be empty to use HEAD).
func WorktreeAdd(repo, branch, path, base string) error {
	args := []string{"-C", repo, "worktree", "add", "-b", branch, path}
	if base != "" {
		args = append(args, base)
	}
	if _, err := run("", args...); err != nil {
		return fmt.Errorf("git worktree add: %w", err)
	}
	return nil
}

// WorktreeRemove removes a worktree from its source repo. repo is the source
// repo path, path is the worktree path to remove.
func WorktreeRemove(repo, path string, force bool) error {
	args := []string{"-C", repo, "worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	if _, err := run("", args...); err != nil {
		return fmt.Errorf("git worktree remove %s: %w", path, err)
	}
	return nil
}

// WorktreePrune cleans up stale worktree metadata for a repo.
func WorktreePrune(repo string) error {
	if _, err := run(repo, "worktree", "prune"); err != nil {
		return fmt.Errorf("git worktree prune: %w", err)
	}
	return nil
}

// DeleteBranch deletes a local branch from repo. force controls -D vs -d.
func DeleteBranch(repo, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	if _, err := run(repo, "branch", flag, branch); err != nil {
		return fmt.Errorf("delete branch %s: %w", branch, err)
	}
	return nil
}

// run executes a git command, optionally with -C repo if repo is non-empty,
// and returns combined stdout+stderr. A non-zero exit yields an error whose
// message includes the captured output.
func run(repo string, args ...string) (string, error) {
	full := args
	if repo != "" {
		full = append([]string{"-C", repo}, args...)
	}
	cmd := exec.Command("git", full...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(out))
		return string(out), fmt.Errorf("%v: %w (%s)", full, err, trimmed)
	}
	return string(out), nil
}
