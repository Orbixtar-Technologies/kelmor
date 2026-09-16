package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenRejectsSymlinkRoot(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(link, "account root"); err == nil {
		t.Fatal("expected symlink root to fail")
	}
}

func TestWriteAtomicRejectsFinalSymlink(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "root")
	if err := Create(rootPath, "test root"); err != nil {
		t.Fatal(err)
	}
	root, err := Open(rootPath, "test root")
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret")
	if err := os.WriteFile(secret, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(rootPath, "target")); err != nil {
		t.Fatal(err)
	}
	_ = root.WriteAtomic("target", []byte("pwn"), 0o640)
	got, err := os.ReadFile(secret)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep" {
		t.Fatalf("outside inode changed: %q", got)
	}
}
