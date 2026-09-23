package operations

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildAuthURL(t *testing.T) {
	got := buildAuthURL("https://github.com/org/repo.git", "secret-token")
	if !strings.Contains(got, "x-access-token:secret-token@") {
		t.Fatalf("expected tokenized URL, got %q", got)
	}
	if buildAuthURL("https://github.com/org/repo.git", "") != "https://github.com/org/repo.git" {
		t.Fatal("empty token must leave URL unchanged")
	}
	if buildAuthURL("git@github.com:org/repo.git", "secret") != "git@github.com:org/repo.git" {
		t.Fatal("SSH URLs must ignore token")
	}
}

func TestGitOpErrorSurfacesStderrAndRedactsToken(t *testing.T) {
	err := gitOpError("git fetch", []byte("fatal: detected dubious ownership\nhttps://x-access-token:sekrit@github.com/o/r.git"), fmt.Errorf("exit status 128"), "sekrit")
	msg := err.Error()
	if !strings.Contains(msg, "dubious ownership") {
		t.Fatalf("expected stderr in error, got %q", msg)
	}
	if strings.Contains(msg, "sekrit") {
		t.Fatalf("token leaked in error: %q", msg)
	}
	if !strings.Contains(msg, "[redacted]") {
		t.Fatalf("expected redaction marker, got %q", msg)
	}
}

func TestGitSafeDirectoryEnvAllowListed(t *testing.T) {
	if err := validateFixedCommand("/usr/bin/git", gitSafeDirectoryEnv(), []string{"fetch"}); err != nil {
		t.Fatalf("safe.directory env must be allow-listed: %v", err)
	}
}

func TestSyncGitWorkDirIntoNonEmptyDirectory(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "hello.txt"), []byte("from-git\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = src
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s %v", args, out, err)
		}
	}
	run("init")
	run("add", ".")
	run("commit", "-m", "init")
	run("branch", "-M", "main")

	h := &Host{Root: t.TempDir()}
	if _, err := h.CreateDirectoryTree("/home/acme/public_html", 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := h.ApplyFile("/home/acme/public_html/index.html", []byte("placeholder"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := h.syncGitWorkDir("/home/acme/public_html", src, "main", ""); err != nil {
		t.Fatalf("sync non-empty: %v", err)
	}
	body, err := h.readManaged("/home/acme/public_html/hello.txt", 1024)
	if err != nil || string(body) != "from-git\n" {
		t.Fatalf("checkout missing: %s %v", body, err)
	}
}

func TestDeployApplicationStaticSPAUnit(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if _, err := h.ApplyFile("/home/acme/app/package.json", []byte(`{"scripts":{"build":"vite build"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := h.deployApplication("site-1", "acme", "node", "/home/acme/app", "")
	if err != nil || result.ObservedState != "configured" {
		t.Fatalf("%+v %v", result, err)
	}
	body, err := h.readManaged("/etc/systemd/system/panel-app-site-1.service", 10000)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "ExecStart=/usr/bin/node "+panelStaticServer) {
		t.Fatalf("unit: %s", body)
	}
	if !strings.Contains(string(body), "STATIC_ROOT=dist") {
		t.Fatalf("missing STATIC_ROOT: %s", body)
	}
	helper, err := h.readManaged(panelStaticServer, 10000)
	if err != nil || !strings.Contains(string(helper), "SOCKET_PATH") {
		t.Fatalf("static helper missing: %s %v", helper, err)
	}
}
