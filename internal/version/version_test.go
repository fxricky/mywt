package version

import "testing"

func TestGet(t *testing.T) {
	orig := version
	defer func() { version = orig }()

	// unset → "dev" (tests are built without ldflags and ReadBuildInfo returns
	// a devel build info)
	version = ""
	if got := Get(); got != "dev" {
		t.Errorf("Get() with empty version = %q, want dev", got)
	}

	// set explicitly
	version = "v9.9.9"
	if got := Get(); got != "v9.9.9" {
		t.Errorf("Get() with explicit version = %q, want v9.9.9", got)
	}
}
