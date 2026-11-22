package session

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDeriveBranchName(t *testing.T) {
	const id = "worktree-20260730123456"
	for _, tc := range []struct {
		name, sessionID, sessionName, branch, prefix string
		prefixSet                                    bool
		want                                         string
	}{
		{name: "unnamed default", sessionID: id, want: "wt-20260730123456"},
		{name: "unnamed custom prefix", sessionID: id, prefix: "feat", want: "feat-20260730123456"},
		{name: "unnamed explicit branch", sessionID: id, branch: "custom-branch", prefix: "feat", want: "custom-branch"},
		{name: "unnamed blank branch", sessionID: id, branch: "  ", want: "wt-20260730123456"},
		{name: "unnamed blank prefix", sessionID: id, prefix: "  ", want: "wt-20260730123456"},
		{name: "collision suffix", sessionID: "worktree-20260730123456-2", want: "wt-20260730123456-2"},
		{name: "named session", sessionID: id, sessionName: "auth-refactor", want: "auth-refactor"},
		{name: "named ignores default prefix", sessionID: id, sessionName: "auth-refactor", prefix: "wt", want: "auth-refactor"},
		{name: "named trimmed", sessionID: id, sessionName: "  auth-refactor  ", want: "auth-refactor"},
		{name: "named explicit branch wins", sessionID: id, sessionName: "auth-refactor", branch: "feature-x", prefix: "feat", prefixSet: true, want: "feature-x"},
		{name: "named explicit prefix uses timestamp", sessionID: id, sessionName: "auth-refactor", prefix: "feat", prefixSet: true, want: "feat-20260730123456"},
		{name: "named non-default prefix is explicit", sessionID: id, sessionName: "auth-refactor", prefix: "feat", want: "feat-20260730123456"},
		{name: "explicit default prefix flag", sessionID: id, sessionName: "auth-refactor", prefix: "wt", prefixSet: true, want: "wt-20260730123456"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveBranchName(tc.sessionID, tc.sessionName, tc.branch, tc.prefix, tc.prefixSet)
			if got != tc.want {
				t.Errorf("deriveBranchName() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidateSubfolder(t *testing.T) {
	for _, tc := range []struct {
		name string
		want string // empty means valid (no error)
	}{
		{"backend", ""},
		{"  backend  ", ""},
		{"", "invalid subfolder name"},
		{"  ", "invalid subfolder name"},
		{"be/fe", "invalid subfolder name"},
		{"be\\fe", "invalid subfolder name"},
		{"a/b", "invalid subfolder name"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSubfolder(tc.name)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("validateSubfolder(%q) unexpected error: %v", tc.name, err)
				}
			} else {
				if err == nil {
					t.Fatalf("validateSubfolder(%q) expected error, got nil", tc.name)
				}
				if !contains(err.Error(), tc.want) {
					t.Errorf("validateSubfolder(%q) error = %q, want containing %q", tc.name, err.Error(), tc.want)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) > 0 && containsImpl(s, substr))
}
func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestManagerCreateAndList(t *testing.T) {
	root := t.TempDir()
	m := New(root)

	s1, err := m.Create(CreateOptions{Name: "feat-a"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if s1.Name != "feat-a" {
		t.Errorf("Name = %q, want feat-a", s1.Name)
	}

	s2, err := m.Create(CreateOptions{})
	if err != nil {
		t.Fatalf("Create second: %v", err)
	}

	list, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len(List) = %d, want 2", len(list))
	}
	// Chronological order (by ID)
	if list[0].ID != s1.ID {
		t.Errorf("list[0].ID = %q, want %q", list[0].ID, s1.ID)
	}
	if list[1].ID != s2.ID {
		t.Errorf("list[1].ID = %q, want %q", list[1].ID, s2.ID)
	}
}

func TestManagerResolve(t *testing.T) {
	root := t.TempDir()
	m := New(root)

	s1, err := m.Create(CreateOptions{Name: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	s2, err := m.Create(CreateOptions{Name: "alpha"}) // same name
	if err != nil {
		t.Fatal(err)
	}
	if s1.ID == s2.ID {
		t.Fatal("expected two different session ids")
	}

	// resolve by full id
	got, err := m.Resolve(s1.ID)
	if err != nil || got.ID != s1.ID {
		t.Fatalf("Resolve(full id): %v, %+v", err, got)
	}

	// resolve by bare timestamp
	ts := s1.ID[len(IDPrefix):]
	got, err = m.Resolve(ts)
	if err != nil || got.ID != s1.ID {
		t.Fatalf("Resolve(timestamp): %v, %+v", err, got)
	}

	// resolve by name (ambiguous → error)
	_, err = m.Resolve("alpha")
	if err == nil {
		t.Fatal("Resolve(alpha) should be ambiguous")
	}

	// not found
	_, err = m.Resolve("nonexistent")
	if err == nil {
		t.Fatal("Resolve(nonexistent) should error")
	}
}

func TestManagerFindByPath(t *testing.T) {
	root := t.TempDir()
	m := New(root)
	s := &Session{
		ID:        "worktree-20260730123456",
		CreatedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		Root:      root,
		Projects: []Project{
			{Subfolder: "backend", Branch: "feature/backend", Path: filepath.Join(root, "worktree-20260730123456", "backend")},
			{Subfolder: "frontend", Branch: "feature/frontend", Path: filepath.Join(root, "worktree-20260730123456", "frontend")},
		},
	}
	if err := s.save(); err != nil {
		t.Fatalf("save session: %v", err)
	}
	for _, p := range s.Projects {
		if err := os.MkdirAll(filepath.Join(p.Path, "src", "api"), 0o755); err != nil {
			t.Fatalf("create project path: %v", err)
		}
	}
	if err := os.MkdirAll(s.Projects[0].Path+"-copy", 0o755); err != nil {
		t.Fatalf("create similar-prefix path: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "elsewhere"), 0o755); err != nil {
		t.Fatalf("create outside path: %v", err)
	}
	symlink := filepath.Join(root, "backend-link")
	if err := os.Symlink(s.Projects[0].Path, symlink); err != nil {
		t.Fatalf("create project symlink: %v", err)
	}

	tests := []struct {
		name        string
		path        string
		wantProject string
		wantFound   bool
	}{
		{name: "project root", path: s.Projects[0].Path, wantProject: "backend", wantFound: true},
		{name: "nested directory", path: filepath.Join(s.Projects[0].Path, "src", "api"), wantProject: "backend", wantFound: true},
		{name: "symlinked project", path: filepath.Join(symlink, "src", "api"), wantProject: "backend", wantFound: true},
		{name: "similar prefix is not a match", path: s.Projects[0].Path + "-copy", wantFound: false},
		{name: "session root is not a project", path: s.Dir(), wantFound: false},
		{name: "outside session", path: filepath.Join(root, "elsewhere"), wantFound: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotSession, gotProject, gotFound, err := m.FindByPath(tc.path)
			if err != nil {
				t.Fatalf("FindByPath: %v", err)
			}
			if gotFound != tc.wantFound {
				t.Fatalf("FindByPath found = %v, want %v", gotFound, tc.wantFound)
			}
			if !tc.wantFound {
				if gotSession != nil {
					t.Fatalf("FindByPath session = %+v, want nil", gotSession)
				}
				return
			}
			if gotSession.ID != s.ID {
				t.Errorf("FindByPath session ID = %q, want %q", gotSession.ID, s.ID)
			}
			if gotProject.Subfolder != tc.wantProject {
				t.Errorf("FindByPath project = %q, want %q", gotProject.Subfolder, tc.wantProject)
			}
		})
	}
}

func TestAddBranchResolution(t *testing.T) {
	t.Run("named session uses the session name in each repo", func(t *testing.T) {
		root := t.TempDir()
		m := New(root)
		backend := initRepo(t)
		frontend := initRepo(t)
		s, err := m.Create(CreateOptions{Name: "auth-refactor"})
		if err != nil {
			t.Fatal(err)
		}

		t.Chdir(backend)
		s, err = m.Add(s, AddOptions{Subfolder: "backend"})
		if err != nil {
			t.Fatal(err)
		}
		t.Chdir(frontend)
		s, err = m.Add(s, AddOptions{Subfolder: "frontend"})
		if err != nil {
			t.Fatal(err)
		}
		if len(s.Projects) != 2 {
			t.Fatalf("projects = %d, want 2", len(s.Projects))
		}
		for _, p := range s.Projects {
			if p.Branch != "auth-refactor" {
				t.Errorf("%s branch = %q, want auth-refactor", p.Subfolder, p.Branch)
			}
			if _, err := os.Stat(p.Path); err != nil {
				t.Errorf("worktree %s: %v", p.Path, err)
			}
		}
		if got := gitRev(t, backend, "auth-refactor"); got == "" {
			t.Fatal("backend branch missing")
		}
		if got := gitRev(t, frontend, "auth-refactor"); got == "" {
			t.Fatal("frontend branch missing")
		}
	})

	t.Run("explicit branch wins and does not create the session name", func(t *testing.T) {
		root := t.TempDir()
		m := New(root)
		repo := initRepo(t)
		s, err := m.Create(CreateOptions{Name: "auth-refactor"})
		if err != nil {
			t.Fatal(err)
		}
		t.Chdir(repo)
		s, err = m.Add(s, AddOptions{Subfolder: "backend", Branch: "feature-x"})
		if err != nil {
			t.Fatal(err)
		}
		if s.Projects[0].Branch != "feature-x" {
			t.Fatalf("branch = %q", s.Projects[0].Branch)
		}
		if gitShowRef(t, repo, "refs/heads/auth-refactor") {
			t.Fatal("session name was created as a branch despite --branch")
		}
		if !gitShowRef(t, repo, "refs/heads/feature-x") {
			t.Fatal("explicit branch was not created")
		}
	})

	t.Run("explicit prefix on a named session keeps the timestamp default", func(t *testing.T) {
		root := t.TempDir()
		m := New(root)
		repo := initRepo(t)
		s, err := m.Create(CreateOptions{Name: "auth-refactor"})
		if err != nil {
			t.Fatal(err)
		}
		t.Chdir(repo)
		s, err = m.Add(s, AddOptions{Subfolder: "backend", BranchPrefix: "feat", BranchPrefixSet: true})
		if err != nil {
			t.Fatal(err)
		}
		want := "feat-" + strings.TrimPrefix(s.ID, IDPrefix)
		if s.Projects[0].Branch != want {
			t.Fatalf("branch = %q, want %q", s.Projects[0].Branch, want)
		}
	})

	t.Run("unnamed session keeps the timestamp default", func(t *testing.T) {
		root := t.TempDir()
		m := New(root)
		repo := initRepo(t)
		s, err := m.Create(CreateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		t.Chdir(repo)
		s, err = m.Add(s, AddOptions{Subfolder: "backend"})
		if err != nil {
			t.Fatal(err)
		}
		want := "wt-" + strings.TrimPrefix(s.ID, IDPrefix)
		if s.Projects[0].Branch != want {
			t.Fatalf("branch = %q, want %q", s.Projects[0].Branch, want)
		}
	})

	t.Run("checked out branch is refused without changes", func(t *testing.T) {
		root := t.TempDir()
		m := New(root)
		repo := initRepo(t)
		before := gitRev(t, repo, "main")
		s, err := m.Create(CreateOptions{Name: "main"})
		if err != nil {
			t.Fatal(err)
		}
		metaBefore, err := os.ReadFile(s.MetaPath())
		if err != nil {
			t.Fatal(err)
		}
		t.Chdir(repo)
		_, err = m.Add(s, AddOptions{Subfolder: "backend"})
		if err == nil || !strings.Contains(err.Error(), "checked out") || !strings.Contains(err.Error(), "--branch") {
			t.Fatalf("err = %v", err)
		}
		if got := gitRev(t, repo, "main"); got != before {
			t.Fatalf("main moved from %s to %s", before, got)
		}
		if _, statErr := os.Stat(filepath.Join(s.Dir(), "backend")); !os.IsNotExist(statErr) {
			t.Fatalf("worktree created despite conflict: %v", statErr)
		}
		metaAfter, err := os.ReadFile(s.MetaPath())
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(metaBefore, metaAfter) {
			t.Fatalf("session metadata changed:\n%s", metaAfter)
		}
	})

	t.Run("existing branch is refused without reset or delete", func(t *testing.T) {
		root := t.TempDir()
		m := New(root)
		repo := initRepo(t)
		runGit(t, repo, "branch", "dormant")
		before := gitRev(t, repo, "dormant")
		s, err := m.Create(CreateOptions{Name: "dormant"})
		if err != nil {
			t.Fatal(err)
		}
		t.Chdir(repo)
		_, err = m.Add(s, AddOptions{Subfolder: "backend"})
		if err == nil || !strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "checked out") {
			t.Fatalf("err = %v", err)
		}
		if got := gitRev(t, repo, "dormant"); got != before {
			t.Fatalf("dormant moved from %s to %s", before, got)
		}
		cmd := exec.Command("git", "-C", repo, "rev-parse", "--abbrev-ref", "HEAD")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("HEAD: %v\n%s", err, out)
		}
		if branch := strings.TrimSpace(string(out)); branch != "main" {
			t.Fatalf("HEAD = %s, want main", branch)
		}
	})

	t.Run("invalid session name creates nothing", func(t *testing.T) {
		root := t.TempDir()
		m := New(root)
		for _, name := range []string{"bad name", "bad..name", "-bad", "HEAD", "bad.lock"} {
			if _, err := m.Create(CreateOptions{Name: name}); err == nil {
				t.Fatalf("Create(%q) succeeded", name)
			}
		}
		list, err := m.List()
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 0 {
			t.Fatalf("len(list) = %d, want 0", len(list))
		}
	})

	t.Run("existing metadata and branches are not rewritten", func(t *testing.T) {
		root := t.TempDir()
		m := New(root)
		repo := initRepo(t)
		s, err := m.Create(CreateOptions{Name: "kept-name"})
		if err != nil {
			t.Fatal(err)
		}
		s.Projects = append(s.Projects, Project{
			Subfolder: "legacy",
			Repo:      "/nowhere",
			Branch:    "wt-20200101010101",
			Path:      "/nowhere/legacy",
		})
		if err := s.save(); err != nil {
			t.Fatal(err)
		}
		t.Chdir(repo)
		s, err = m.Add(s, AddOptions{Subfolder: "backend"})
		if err != nil {
			t.Fatal(err)
		}
		reloaded, err := m.Resolve(s.ID)
		if err != nil {
			t.Fatal(err)
		}
		legacy, ok := reloaded.FindProject("legacy")
		if !ok || legacy.Branch != "wt-20200101010101" {
			t.Fatalf("legacy project = %+v, ok=%v", legacy, ok)
		}
		backend, ok := reloaded.FindProject("backend")
		if !ok || backend.Branch != "kept-name" {
			t.Fatalf("backend project = %+v, ok=%v", backend, ok)
		}
		if gitShowRef(t, repo, "refs/heads/wt-20200101010101") {
			t.Fatal("legacy metadata branch was created in the new repo")
		}
	})

	t.Run("invalid explicit branch does not create a worktree", func(t *testing.T) {
		root := t.TempDir()
		m := New(root)
		repo := initRepo(t)
		s, err := m.Create(CreateOptions{Name: "auth-refactor"})
		if err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(s.MetaPath())
		if err != nil {
			t.Fatal(err)
		}
		t.Chdir(repo)
		_, err = m.Add(s, AddOptions{Subfolder: "backend", Branch: "bad..name"})
		if err == nil || !strings.Contains(err.Error(), "invalid branch name") {
			t.Fatalf("err = %v", err)
		}
		after, err := os.ReadFile(s.MetaPath())
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatal("metadata changed")
		}
		if _, statErr := os.Stat(filepath.Join(s.Dir(), "backend")); !os.IsNotExist(statErr) {
			t.Fatalf("worktree created: %v", statErr)
		}
	})

	t.Run("handwritten invalid name is not rewritten", func(t *testing.T) {
		root := t.TempDir()
		m := New(root)
		repo := initRepo(t)
		s := &Session{
			ID:        "worktree-20200101010101",
			Name:      "bad name",
			CreatedAt: time.Now().UTC(),
			Root:      root,
		}
		if err := s.save(); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(s.MetaPath())
		if err != nil {
			t.Fatal(err)
		}
		t.Chdir(repo)
		_, err = m.Add(s, AddOptions{Subfolder: "backend"})
		if err == nil || !strings.Contains(err.Error(), "--branch") {
			t.Fatalf("err = %v", err)
		}
		after, err := os.ReadFile(s.MetaPath())
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatal("metadata changed")
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

func gitShowRef(t *testing.T, dir, ref string) bool {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "show-ref", "--verify", "--quiet", ref)
	return cmd.Run() == nil
}

func TestUniqueIDRapidCreates(t *testing.T) {
	root := t.TempDir()
	m := New(root)

	// Create multiple sessions as rapidly as possible.
	var ids []string
	for i := 0; i < 3; i++ {
		s, err := m.Create(CreateOptions{})
		if err != nil {
			t.Fatalf("Create #%d: %v", i+1, err)
		}
		ids = append(ids, s.ID)
	}

	// All IDs must be unique (collision handling worked).
	seen := make(map[string]bool)
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("duplicate session id: %q", id)
		}
		seen[id] = true
	}
}
