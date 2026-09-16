package bundle_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallScriptRequiresPayloadAndUbuntu(t *testing.T) {
	script, err := filepath.Abs("install.sh")
	if err != nil {
		t.Fatal(err)
	}
	empty := t.TempDir()
	copyScript(t, script, empty)
	out, err := runInstall(empty, nil)
	if err == nil || !strings.Contains(out, "missing") {
		t.Fatalf("empty tree: err=%v out=%s", err, out)
	}

	partial := t.TempDir()
	copyScript(t, script, partial)
	if err := os.MkdirAll(filepath.Join(partial, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(partial, "bin", "panel-install"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err = runInstall(partial, []string{"--dev", "--non-interactive"})
	if err == nil || !strings.Contains(out, "portals") {
		t.Fatalf("missing portals: err=%v out=%s", err, out)
	}

	ready := stageTree(t, script)
	root := filepath.Join(t.TempDir(), "host")
	if err := os.MkdirAll(filepath.Join(root, "etc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "etc", "os-release"), []byte("NAME=\"Debian GNU/Linux\"\nVERSION_ID=\"12\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = runInstall(ready, []string{"--root", root, "--non-interactive"})
	if err == nil || !strings.Contains(out, "Ubuntu 24.04") {
		t.Fatalf("wrong OS: err=%v out=%s", err, out)
	}
}

func TestInstallScriptStagesAndExecsPanelInstall(t *testing.T) {
	script, err := filepath.Abs("install.sh")
	if err != nil {
		t.Fatal(err)
	}
	ready := stageTree(t, script)
	root := filepath.Join(t.TempDir(), "host")
	if err := os.MkdirAll(filepath.Join(root, "etc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "etc", "os-release"), []byte("NAME=\"Ubuntu\"\nVERSION_ID=\"24.04\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runInstall(ready, []string{"--root", root, "--non-interactive", "--hostname", "panel.example.net"})
	if err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	if !strings.Contains(out, "panel-install") || !strings.Contains(out, "--hostname") {
		t.Fatalf("did not exec panel-install: %s", out)
	}
	if _, err := os.Stat(filepath.Join(root, "usr/local/panel/bin/panel-install")); err != nil {
		t.Fatal(err)
	}
	release, err := os.ReadFile(filepath.Join(root, "usr/local/panel/current-release"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(release)) != "0.2.325-test" {
		t.Fatalf("current-release %q", release)
	}
	if _, err := os.Stat(filepath.Join(root, "usr/local/panel/share/portals/server/index.html")); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "usr/local/bin/panel-install")
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("panel-install PATH link: %v", err)
	}
	if target != filepath.Join(root, "usr/local/panel/bin/panel-install") {
		t.Fatalf("panel-install link %q", target)
	}
}

func TestInstallScriptReplacesBusyBinary(t *testing.T) {
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep not available")
	}
	script, err := filepath.Abs("install.sh")
	if err != nil {
		t.Fatal(err)
	}
	ready := stageTree(t, script)
	if err := os.WriteFile(filepath.Join(ready, "bin", "panel-api"), []byte("#!/bin/sh\necho replaced\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "host")
	dest := filepath.Join(root, "usr/local/panel/bin")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "etc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "etc", "os-release"), []byte("NAME=\"Ubuntu\"\nVERSION_ID=\"24.04\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(sleep)
	if err != nil {
		t.Fatal(err)
	}
	busy := filepath.Join(dest, "panel-api")
	if err := os.WriteFile(busy, body, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(busy, "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	out, err := runInstall(ready, []string{"--root", root, "--non-interactive", "--hostname", "panel.example.net"})
	if err != nil {
		t.Fatalf("busy replace: %v\n%s", err, out)
	}
	got, err := os.ReadFile(busy)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "replaced") {
		t.Fatalf("running binary was not replaced: %q", got)
	}
}

func stageTree(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "share/portals/server"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "share/portals/account"), 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "install.sh"), body, 0o755); err != nil {
		t.Fatal(err)
	}
	stub := "#!/bin/sh\nprintf 'panel-install %s\\n' \"$*\"\n"
	if err := os.WriteFile(filepath.Join(dir, "bin", "panel-install"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "share/portals/server/index.html"), []byte("<!doctype html><title>Kelmor Director</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "share/portals/account/index.html"), []byte("<!doctype html><title>Kelmor Control</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("0.2.325-test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func copyScript(t *testing.T, script, dest string) {
	t.Helper()
	body, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "install.sh"), body, 0o755); err != nil {
		t.Fatal(err)
	}
}

func runInstall(tree string, args []string) (string, error) {
	cmdArgs := append([]string{filepath.Join(tree, "install.sh")}, args...)
	cmd := exec.Command("bash", cmdArgs...)
	cmd.Dir = tree
	body, err := cmd.CombinedOutput()
	return string(body), err
}
