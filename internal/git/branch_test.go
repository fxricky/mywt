package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateBranchName(t *testing.T) {
	valid := []string{"auth-refactor", "feature/auth", "wt-20260730123456"}
	for _, name := range valid {
		t.Run("valid "+name, func(t *testing.T) {
			if err := ValidateBranchName(name); err != nil {
				t.Fatalf("ValidateBranchName(%q) = %v", name, err)
			}
		})
	}

	invalid := []string{"", " ", "bad name", "bad..name", "-bad", "HEAD", "bad.lock", "a/", "a."}
	for _, name := range invalid {
		t.Run("invalid "+name, func(t *testing.T) {
			err := ValidateBranchName(name)
			if err == nil {
				t.Fatalf("ValidateBranchName(%q) succeeded", name)
			}
			if !strings.Contains(err.Error(), "invalid git branch name") {
				t.Fatalf("ValidateBranchName(%q) = %v", name, err)
			}
		})
	}
}

func TestRefuseExistingBranch(t *testing.T) {
	repo := initRepo(t)

	t.Run("available", func(t *testing.T) {
		if err := RefuseExistingBranch(repo, "auth-refactor"); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("checked out", func(t *testing.T) {
		before := gitRev(t, repo, "main")
		trees := worktreeCount(t, repo)
		err := RefuseExistingBranch(repo, "main")
		if err == nil {
			t.Fatal("expected a conflict")
		}
		if !strings.Contains(err.Error(), "checked out") || !strings.Contains(err.Error(), "--branch") {
			t.Fatalf("error = %v", err)
		}
		if !strings.Contains(err.Error(), "will not reuse, reset, move, or delete") {
			t.Fatalf("error = %v", err)
		}
		if got := gitRev(t, repo, "main"); got != before {
			t.Fatalf("main moved from %s to %s", before, got)
		}
		if got := worktreeCount(t, repo); got != trees {
			t.Fatalf("worktree count = %d, want %d", got, trees)
		}
	})

	t.Run("exists but not checked out", func(t *testing.T) {
		runGit(t, repo, "branch", "dormant")
		before := gitRev(t, repo, "dormant")
		err := RefuseExistingBranch(repo, "dormant")
		if err == nil {
			t.Fatal("expected a conflict")
		}
		if strings.Contains(err.Error(), "checked out") {
			t.Fatalf("dormant branch reported as checked out: %v", err)
		}
		if !strings.Contains(err.Error(), "already exists") || !strings.Contains(err.Error(), "--branch") {
			t.Fatalf("error = %v", err)
		}
		if got := gitRev(t, repo, "dormant"); got != before {
			t.Fatalf("dormant moved from %s to %s", before, got)
		}
		if branch, err := CurrentBranch(repo); err != nil || branch != "main" {
			t.Fatalf("HEAD = %q, %v; want main", branch, err)
		}
	})
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "--quiet", "-b", "main")
	runGit(t, dir, "config", "user.email", "mywt-test@example.com")
	runGit(t, dir, "config", "user.name", "mywt test")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "README")
	runGit(t, dir, "commit", "--quiet", "-m", "init")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func gitRev(t *testing.T, dir, rev string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "rev-parse", rev)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse %s: %v\n%s", rev, err, out)
	}
	return strings.TrimSpace(string(out))
}

func worktreeCount(t *testing.T, repo string) int {
	t.Helper()
	out, err := exec.Command("git", "-C", repo, "worktree", "list", "--porcelain").CombinedOutput()
	if err != nil {
		t.Fatalf("worktree list: %v\n%s", err, out)
	}
	n := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			n++
		}
	}
	return n
}
