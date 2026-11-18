package version

import "testing"

func TestGet(t *testing.T) {
	orig := version
	defer func() { version = orig }()

	if defaultVersion != "v0.3.0" {
		t.Fatalf("defaultVersion = %q, want v0.3.0", defaultVersion)
	}

	// unset → defaultVersion (tests are built without ldflags and ReadBuildInfo
	// returns a devel build info)
	version = ""
	if got := Get(); got != "v0.3.0" {
		t.Errorf("Get() with empty version = %q, want v0.3.0", got)
	}

	// set explicitly
	version = "v9.9.9"
	if got := Get(); got != "v9.9.9" {
		t.Errorf("Get() with explicit version = %q, want v9.9.9", got)
	}
}

func TestReleaseVersion(t *testing.T) {
	for _, v := range []string{"", "(devel)", "v0.2.0+dirty", "v0.2.0-0.20261118120000-abcdef"} {
		if got, ok := releaseVersion(v); ok {
			t.Errorf("releaseVersion(%q) = %q, %v; want unset", v, got, ok)
		}
	}
	got, ok := releaseVersion("v0.3.0")
	if !ok || got != "v0.3.0" {
		t.Fatalf("releaseVersion(v0.3.0) = %q, %v", got, ok)
	}
}
