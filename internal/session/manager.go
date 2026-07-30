package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fxricky/mywt/internal/git"
)

// Manager performs session operations against a fixed worktree root.
type Manager struct {
	Root string
}

// New returns a Manager for the given (already expanded) root.
func New(root string) *Manager {
	return &Manager{Root: root}
}

// CreateOptions controls Create.
type CreateOptions struct {
	Name string
}

// Create makes a new empty timestamped session and returns it.
func (m *Manager) Create(opts CreateOptions) (*Session, error) {
	if err := os.MkdirAll(m.Root, 0o755); err != nil {
		return nil, fmt.Errorf("create worktree root: %w", err)
	}

	id, err := m.uniqueID()
	if err != nil {
		return nil, err
	}
	s := &Session{
		ID:        id,
		Name:      strings.TrimSpace(opts.Name),
		CreatedAt: time.Now().UTC(),
		Root:      m.Root,
	}
	if err := s.save(); err != nil {
		return nil, err
	}
	return s, nil
}

// uniqueID returns a worktree-<YYYYMMDDHHmmss> id that does not collide with an
// existing session dir. If a collision occurs (same-second create) it appends
// -2, -3, ... to the timestamp.
func (m *Manager) uniqueID() (string, error) {
	base := IDPrefix + time.Now().Format("20060102150405")
	id := base
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(m.Root, id)); err != nil {
			if os.IsNotExist(err) {
				return id, nil
			}
			return "", fmt.Errorf("stat session dir: %w", err)
		}
		id = fmt.Sprintf("%s-%d", base, i)
		if i > 999 {
			return "", fmt.Errorf("could not allocate unique session id")
		}
	}
}

// AddOptions controls Add.
type AddOptions struct {
	// Subfolder overrides the project subfolder name (default: basename of repo).
	Subfolder string
	// Branch overrides the new branch name. If empty, BranchPrefix+timestamp is used.
	Branch string
	// BranchPrefix is prepended to the session timestamp to form the branch
	// name when Branch is empty. Defaults to "wt".
	BranchPrefix string
	// Base is the ref to start the new branch from. If empty, the repo's
	// default branch is auto-detected.
	Base string
}

// Add creates a worktree of the repo enclosing cwd and registers it as a
// project in the given session. It must be run from inside a git repo.
func (m *Manager) Add(s *Session, opts AddOptions) (*Session, error) {
	repo, err := git.RepoRoot()
	if err != nil {
		return nil, err
	}

	subfolder := strings.TrimSpace(opts.Subfolder)
	if subfolder == "" {
		subfolder = filepath.Base(repo)
	}
	if subfolder == "" || strings.ContainsAny(subfolder, `/\`) {
		return nil, fmt.Errorf("invalid subfolder name %q", subfolder)
	}
	if _, exists := s.FindProject(subfolder); exists {
		return nil, fmt.Errorf("project %q already exists in session %s (use --as to rename)", subfolder, s.ID)
	}

	branch := strings.TrimSpace(opts.Branch)
	if branch == "" {
		prefix := strings.TrimSpace(opts.BranchPrefix)
		if prefix == "" {
			prefix = "wt"
		}
		ts := strings.TrimPrefix(s.ID, IDPrefix)
		if prefix == "" {
			branch = ts
		} else {
			branch = prefix + "-" + ts
		}
	}

	base := strings.TrimSpace(opts.Base)
	if base == "" {
		base, err = git.DefaultBranch(repo)
		if err != nil {
			return nil, err
		}
	}

	dest := filepath.Join(s.Dir(), subfolder)
	if _, err := os.Stat(dest); err == nil {
		return nil, fmt.Errorf("destination already exists: %s", dest)
	}

	if err := git.WorktreeAdd(repo, branch, dest, base); err != nil {
		return nil, err
	}

	absDest, _ := filepath.Abs(dest)
	s.Projects = append(s.Projects, Project{
		Subfolder: subfolder,
		Repo:      repo,
		Branch:    branch,
		Path:      absDest,
	})
	if err := s.save(); err != nil {
		return nil, err
	}
	return s, nil
}

// List returns all sessions found under the root, sorted by id (chronological).
// Missing .session.json dirs are skipped.
func (m *Manager) List() ([]*Session, error) {
	entries, err := os.ReadDir(m.Root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read worktree root: %w", err)
	}
	var sessions []*Session
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !strings.HasPrefix(e.Name(), IDPrefix) {
			continue
		}
		s, err := loadSession(filepath.Join(m.Root, e.Name()))
		if err != nil {
			continue // skip broken/empty session dirs
		}
		sessions = append(sessions, s)
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].ID < sessions[j].ID })
	return sessions, nil
}

// Resolve finds a single session matching idOrName. A match can be the full id
// (worktree-<ts>), the bare timestamp, or the session's name. It returns an
// error if no session or more than one session matches.
func (m *Manager) Resolve(idOrName string) (*Session, error) {
	sessions, err := m.List()
	if err != nil {
		return nil, err
	}
	var matches []*Session
	for _, s := range sessions {
		switch {
		case s.ID == idOrName:
			matches = append(matches, s)
		case s.ID == IDPrefix+idOrName: // bare timestamp given
			matches = append(matches, s)
		case s.Name != "" && s.Name == idOrName:
			matches = append(matches, s)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no session matches %q", idOrName)
	case 1:
		return matches[0], nil
	default:
		var ids []string
		for _, s := range matches {
			ids = append(ids, s.ID)
		}
		return nil, fmt.Errorf("ambiguous session %q matches: %s", idOrName, strings.Join(ids, ", "))
	}
}

// RemoveOptions controls Remove.
type RemoveOptions struct {
	Force          bool
	DeleteBranches bool
}

// Remove removes every project worktree from its source repo, then deletes the
// session directory. Branches are kept unless DeleteBranches is set.
func (m *Manager) Remove(s *Session, opts RemoveOptions) error {
	for _, p := range s.Projects {
		if err := git.WorktreeRemove(p.Repo, p.Path, opts.Force); err != nil {
			// If the worktree folder is already gone, try to prune and continue.
			if _, statErr := os.Stat(p.Path); os.IsNotExist(statErr) {
				_ = git.WorktreePrune(p.Repo)
				continue
			}
			return err
		}
		if opts.DeleteBranches {
			if err := git.DeleteBranch(p.Repo, p.Branch, true); err != nil {
				fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			}
		}
	}
	if err := os.RemoveAll(s.Dir()); err != nil {
		return fmt.Errorf("remove session dir: %w", err)
	}
	return nil
}

// PruneResult describes what a prune pass would do.
type PruneResult struct {
	PrunedRepos    []string // repos that had git worktree prune run
	BrokenSessions []string // session dirs removed (or to remove) for missing meta
}

// Prune runs `git worktree prune` on every referenced repo and removes session
// directories that lack .session.json. When dryRun is true nothing is deleted;
// the result describes what would happen.
func (m *Manager) Prune(dryRun bool) (*PruneResult, error) {
	res := &PruneResult{}

	// Collect unique repos across all sessions.
	seen := map[string]bool{}
	sessions, _ := m.List()
	for _, s := range sessions {
		for _, p := range s.Projects {
			if !seen[p.Repo] {
				seen[p.Repo] = true
				if !dryRun {
					_ = git.WorktreePrune(p.Repo)
				}
				res.PrunedRepos = append(res.PrunedRepos, p.Repo)
			}
		}
	}

	// Remove session dirs without .session.json.
	entries, err := os.ReadDir(m.Root)
	if err != nil {
		if os.IsNotExist(err) {
			return res, nil
		}
		return res, fmt.Errorf("read worktree root: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), IDPrefix) {
			continue
		}
		dir := filepath.Join(m.Root, e.Name())
		if _, err := os.Stat(filepath.Join(dir, MetaFile)); err != nil {
			if os.IsNotExist(err) {
				res.BrokenSessions = append(res.BrokenSessions, dir)
				if !dryRun {
					_ = os.RemoveAll(dir)
				}
			}
		}
	}
	return res, nil
}
