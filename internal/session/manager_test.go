package session

import (
	"testing"
)

func TestDeriveBranchName(t *testing.T) {
	for _, tc := range []struct {
		sessionID, branch, prefix string
		want                      string
	}{
		{"worktree-20260730123456", "", "", "wt-20260730123456"},
		{"worktree-20260730123456", "", "feat", "feat-20260730123456"},
		{"worktree-20260730123456", "custom-branch", "", "custom-branch"},
		{"worktree-20260730123456", "custom-branch", "feat", "custom-branch"},
		{"worktree-20260730123456", "", "", "wt-20260730123456"},
		{"worktree-20260730123456", "  ", "", "wt-20260730123456"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			got := deriveBranchName(tc.sessionID, tc.branch, tc.prefix)
			if got != tc.want {
				t.Errorf("deriveBranchName(%q,%q,%q) = %q, want %q",
					tc.sessionID, tc.branch, tc.prefix, got, tc.want)
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
