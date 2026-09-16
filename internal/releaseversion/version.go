package releaseversion

import (
	"os"
	"strings"
)

// Version is the compiled-in release identity. The Makefile overrides it
// with the same value used for the Debian package and signed feed.
var Version = "0.2.0"

// Current returns the single release identity for API, installer, backup,
// package metadata, current-release, and the update feed.
func Current() string {
	if version := strings.TrimSpace(os.Getenv("PANEL_RELEASE_VERSION")); version != "" {
		return version
	}
	return strings.TrimSpace(Version)
}
