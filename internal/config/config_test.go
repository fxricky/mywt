package config

import (
	"path/filepath"
	"testing"
)

func TestRoot(t *testing.T) {
	// Override HOME so the test is hermetic and doesn't depend on the user's
	// real home directory.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	got, err := Root()
	if err != nil {
		t.Fatalf("Root: %v", err)
	}
	want := filepath.Join(tmp, DefaultRootName)
	if got != want {
		t.Errorf("Root() = %q, want %q", got, want)
	}
}
