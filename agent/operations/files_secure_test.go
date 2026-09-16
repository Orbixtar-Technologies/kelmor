package operations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyFileRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret")
	if err := os.WriteFile(secret, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(home, "acme42")); err != nil {
		t.Fatal(err)
	}
	h := &Host{Root: root}
	if _, err := h.ApplyFile("/home/acme42/stolen", []byte("pwn"), 0o640); err == nil {
		t.Fatal("expected symlink escape to fail")
	}
	got, err := os.ReadFile(secret)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep" {
		t.Fatalf("inode outside the sandbox changed: %q", got)
	}
	if _, err := os.Stat(filepath.Join(outside, "stolen")); err == nil {
		t.Fatal("created a file outside the account root")
	}
}

func TestApplyFileRejectsParentSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "note.txt"), []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "home"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "home", "acme42")); err != nil {
		t.Fatal(err)
	}
	h := &Host{Root: root}
	read, err := h.readManagedFile("/home/acme42/note.txt")
	if err == nil {
		t.Fatalf("expected parent symlink read to fail, got %q", read.Message)
	}
}

func TestApplyFileRejectsStagingSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret")
	if err := os.WriteFile(secret, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "var", "lib", "panel", "backups", "staging")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(dir, "a.bin.staging")); err != nil {
		t.Fatal(err)
	}
	h := &Host{Root: root}
	_, _ = h.applyFileChunk("/var/lib/panel/backups/staging/a.bin", []byte("pwned"), 0o640, 0, true)
	got, err := os.ReadFile(secret)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep" {
		t.Fatalf("staging symlink overwrote %q", got)
	}
}

func TestRenameRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	home := filepath.Join(root, "home", "acme42")
	if err := os.MkdirAll(filepath.Join(home, "public_html"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "public_html", "note.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(home, "escape")); err != nil {
		t.Fatal(err)
	}
	h := &Host{Root: root}
	if _, err := h.renameManagedPath("/home/acme42/public_html/note.txt", "/home/acme42/escape/stolen.txt"); err == nil {
		t.Fatal("expected rename through symlink to fail")
	}
	if _, err := os.Stat(filepath.Join(outside, "stolen.txt")); err == nil {
		t.Fatal("renamed a file outside the account root")
	}
}
