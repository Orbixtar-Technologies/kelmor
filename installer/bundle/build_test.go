package bundle_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildInstallerWritesTarball(t *testing.T) {
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	fake := t.TempDir()
	if err := os.MkdirAll(filepath.Join(fake, "dist/bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(fake, "dist/share/portals/server"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(fake, "dist/share/portals/account"), 0o755); err != nil {
		t.Fatal(err)
	}
	stub := []byte("#!/bin/sh\nexit 0\n")
	if err := os.WriteFile(filepath.Join(fake, "dist/bin/panel-install"), stub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fake, "dist/share/portals/server/index.html"), []byte("<!doctype html><title>Kelmor Director</title>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fake, "dist/share/portals/account/index.html"), []byte("<!doctype html><title>Kelmor Control</title>\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stage := filepath.Join(t.TempDir(), "kelmor-installer_0.0.0-test_linux_amd64")
	out := filepath.Join(t.TempDir(), "kelmor-installer_0.0.0-test_linux_amd64.tar.gz")
	cmd := exec.Command("bash", filepath.Join(repo, "scripts/build-installer.sh"))
	cmd.Env = append(os.Environ(),
		"PANEL_INSTALLER_ROOT="+fake,
		"PANEL_INSTALLER_VERSION=0.0.0-test",
		"PANEL_INSTALLER_STAGE="+stage,
		"PANEL_INSTALLER_OUT="+out,
	)
	body, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("builder: %v\n%s", err, body)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(stage, "install.sh")); err != nil {
		t.Fatal(err)
	}
	ver, err := os.ReadFile(filepath.Join(stage, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(ver)) != "0.0.0-test" {
		t.Fatalf("VERSION %q", ver)
	}
	if _, err := os.Stat(filepath.Join(stage, "bin/panel-install")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(stage, "share/portals/server/index.html")); err != nil {
		t.Fatal(err)
	}
	stable := filepath.Join(filepath.Dir(out), "kelmor-installer-linux-amd64.tar.gz")
	if _, err := os.Stat(stable); err != nil {
		if _, armErr := os.Stat(filepath.Join(filepath.Dir(out), "kelmor-installer-linux-arm64.tar.gz")); armErr != nil {
			t.Fatalf("stable tarball: %v / %v", err, armErr)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(out), "get-kelmor.sh")); err != nil {
		t.Fatal(err)
	}
}
