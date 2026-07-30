package session

import (
	"path/filepath"
	"testing"
)

func TestSessionPaths(t *testing.T) {
	s := &Session{
		ID:   "worktree-20260730123456",
		Root: "/Users/test/.mywt",
		Projects: []Project{
			{Subfolder: "backend", Path: "/Users/test/.mywt/worktree-20260730123456/backend"},
			{Subfolder: "fe", Path: "/Users/test/.mywt/worktree-20260730123456/fe"},
		},
	}

	wantDir := filepath.Join("/Users/test/.mywt", "worktree-20260730123456")
	if got := s.Dir(); got != wantDir {
		t.Errorf("Dir() = %q, want %q", got, wantDir)
	}

	wantMeta := filepath.Join(wantDir, ".session.json")
	if got := s.MetaPath(); got != wantMeta {
		t.Errorf("MetaPath() = %q, want %q", got, wantMeta)
	}

	for _, tc := range []struct {
		sub    string
		want   string
		wantOk bool
	}{
		{"backend", "/Users/test/.mywt/worktree-20260730123456/backend", true},
		{"fe", "/Users/test/.mywt/worktree-20260730123456/fe", true},
		{"api", "", false},
	} {
		t.Run(tc.sub, func(t *testing.T) {
			got, ok := s.ProjectPath(tc.sub)
			if ok != tc.wantOk {
				t.Fatalf("ProjectPath(%q) ok = %v, want %v", tc.sub, ok, tc.wantOk)
			}
			if got != tc.want {
				t.Errorf("ProjectPath(%q) = %q, want %q", tc.sub, got, tc.want)
			}
		})
	}
}

func TestSessionFindProject(t *testing.T) {
	s := &Session{
		Projects: []Project{
			{Subfolder: "be", Branch: "wt-123"},
			{Subfolder: "fe", Branch: "custom"},
		},
	}

	p, ok := s.FindProject("fe")
	if !ok || p.Branch != "custom" {
		t.Fatalf("FindProject(fe) = %+v, ok=%v", p, ok)
	}

	_, ok = s.FindProject("ui")
	if ok {
		t.Fatal("FindProject(ui) should not exist")
	}
}
