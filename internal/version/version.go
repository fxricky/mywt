package version

import (
	"runtime/debug"
	"strings"
)

// defaultVersion is reported for a local build that does not inject a version
// and was not installed as a module version. Release builds override version
// via ldflags (see scripts/release.sh). `go install github.com/fxricky/mywt@<tag>`
// leaves version empty and uses the module version from build info instead.
const defaultVersion = "v0.3.0"

// version is overridden at build time via ldflags, e.g.:
//
//	go build -ldflags "-X github.com/fxricky/mywt/internal/version.version=v0.3.0"
var version = ""

// Get returns the current version of the binary. It prefers the ldflags-injected
// value, then a release module version recorded in the build info (set by
// `go install ...@<version>`), and finally defaultVersion. A local checkout
// stamped with a dirty or pseudo-version still reports defaultVersion.
func Get() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v, ok := releaseVersion(info.Main.Version); ok {
			return v
		}
	}
	return defaultVersion
}

// releaseVersion reports whether v is a module version installed as a release,
// rather than a local build stamped (devel), +dirty, or as a pseudo-version.
func releaseVersion(v string) (string, bool) {
	switch {
	case v == "", v == "(devel)", strings.Contains(v, "+"), strings.Contains(v, "-0."):
		return "", false
	default:
		return v, true
	}
}
