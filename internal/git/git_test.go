package git

import "testing"

func TestCurrentBranchReflectsRename(t *testing.T) {
	repo := t.TempDir()
	if _, err := run(repo, "init", "--quiet"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if _, err := run(repo, "checkout", "--quiet", "-b", "feature-before"); err != nil {
		t.Fatalf("git checkout: %v", err)
	}

	branch, err := CurrentBranch(repo)
	if err != nil {
		t.Fatalf("CurrentBranch before rename: %v", err)
	}
	if branch != "feature-before" {
		t.Fatalf("CurrentBranch before rename = %q, want feature-before", branch)
	}

	if _, err := run(repo, "branch", "-m", "feature-after"); err != nil {
		t.Fatalf("git branch -m: %v", err)
	}
	branch, err = CurrentBranch(repo)
	if err != nil {
		t.Fatalf("CurrentBranch after rename: %v", err)
	}
	if branch != "feature-after" {
		t.Fatalf("CurrentBranch after rename = %q, want feature-after", branch)
	}

	for _, args := range [][]string{
		{"config", "user.name", "mywt test"},
		{"config", "user.email", "mywt-test@example.com"},
		{"commit", "--quiet", "--allow-empty", "-m", "initial"},
		{"checkout", "--quiet", "--detach", "HEAD"},
	} {
		if _, err := run(repo, args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	branch, err = CurrentBranch(repo)
	if err != nil {
		t.Fatalf("CurrentBranch detached: %v", err)
	}
	if branch != "HEAD" {
		t.Fatalf("CurrentBranch detached = %q, want HEAD", branch)
	}
}
