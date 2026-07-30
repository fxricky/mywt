package version

import (
	"runtime/debug"
)

// version is overridden at build time via ldflags, e.g.:
//
//	go build -ldflags "-X github.com/fxricky/mywt/internal/version.version=v0.1.0"
//
// When built with `go install github.com/fxricky/mywt@v0.1.0` the version is
// left empty here and resolved from the build info instead.
var version = ""

// Get returns the current version of the binary. It prefers the ldflags-injected
// value, then the module version recorded in the build info (set by
// `go install ...@<version>`), and finally falls back to "dev".
func Get() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return "dev"
}
