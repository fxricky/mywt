package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultRootName is the fixed folder under the home directory where all
// worktree sessions live.
const DefaultRootName = ".mywt"

// Root returns the fixed worktree root, always ~/.mywt.
func Root() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, DefaultRootName), nil
}
