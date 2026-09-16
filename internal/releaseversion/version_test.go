package releaseversion

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCurrentPrefersExplicitOverride(t *testing.T) {
	t.Setenv("PANEL_RELEASE_VERSION", "9.8.7")
	if got := Current(); got != "9.8.7" {
		t.Fatalf("Current() = %q", got)
	}
}

func TestDefaultMatchesReleaseManifest(t *testing.T) {
	t.Setenv("PANEL_RELEASE_VERSION", "")
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
	body, err := os.ReadFile(filepath.Join(root, "release-manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var platform string
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "platform_release:") {
			platform = strings.TrimSpace(strings.TrimPrefix(line, "platform_release:"))
			break
		}
	}
	if platform == "" {
		t.Fatal("platform_release missing")
	}
	if Current() != platform {
		t.Fatalf("Current() = %q, platform_release = %q", Current(), platform)
	}
}

func TestMakefileInjectsReleaseVersion(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
	body, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "internal/releaseversion.Version") {
		t.Fatal("build does not inject the shared release identity")
	}
}
