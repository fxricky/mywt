package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// IDPrefix is the prefix for session folder ids, e.g. worktree-<ts>.
	IDPrefix = "worktree-"
	// MetaFile is the session metadata filename stored inside the session dir.
	MetaFile = ".session.json"
)

// Project describes a single worktree inside a session.
type Project struct {
	Subfolder string `json:"subfolder"`
	Repo      string `json:"repo"`
	Branch    string `json:"branch"`
	Path      string `json:"path"`
}

// Session is the on-disk metadata for a worktree session.
type Session struct {
	ID        string    `json:"id"`
	Name      string    `json:"name,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Root      string    `json:"root"`
	Projects  []Project `json:"projects"`
}

// Dir returns the absolute path of the session folder.
func (s *Session) Dir() string {
	return filepath.Join(s.Root, s.ID)
}

// MetaPath returns the absolute path of the session metadata file.
func (s *Session) MetaPath() string {
	return filepath.Join(s.Dir(), MetaFile)
}

// ProjectPath returns the absolute path of a project subfolder, or "" if the
// project subfolder is not present in the session.
func (s *Session) ProjectPath(subfolder string) (string, bool) {
	for _, p := range s.Projects {
		if p.Subfolder == subfolder {
			return p.Path, true
		}
	}
	return "", false
}

// FindProject returns the project with the given subfolder, if any.
func (s *Session) FindProject(subfolder string) (Project, bool) {
	for _, p := range s.Projects {
		if p.Subfolder == subfolder {
			return p, true
		}
	}
	return Project{}, false
}

// loadSession reads and parses the .session.json at dir.
func loadSession(dir string) (*Session, error) {
	data, err := os.ReadFile(filepath.Join(dir, MetaFile))
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", MetaFile, err)
	}
	return &s, nil
}

// save writes the session metadata to its Dir().
func (s *Session) save() error {
	if err := os.MkdirAll(s.Dir(), 0o755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	if err := os.WriteFile(s.MetaPath(), data, 0o644); err != nil {
		return fmt.Errorf("write session meta: %w", err)
	}
	return nil
}
