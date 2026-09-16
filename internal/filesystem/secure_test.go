package filesystem

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
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

func TestOpenAcceptsBindMountedAccountRoot(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("bind mount requires root")
	}
	parent := t.TempDir()
	backing := t.TempDir()
	home := filepath.Join(parent, "acme42")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mount(backing, home, "", unix.MS_BIND, ""); err != nil {
		t.Skipf("bind mount unavailable: %v", err)
	}
	t.Cleanup(func() { _ = unix.Unmount(home, unix.MNT_DETACH) })

	parentRoot, err := Open(parent, "home parent")
	if err != nil {
		t.Fatal(err)
	}
	defer parentRoot.Close()
	_, nestedErr := OpenNested(parentRoot, "acme42", "account root")
	if nestedErr == nil {
		t.Fatal("OpenNested must reject a bind-mounted account home")
	}
	if !errors.Is(nestedErr, unix.EXDEV) &&
		!strings.Contains(nestedErr.Error(), "cross-device") {
		t.Fatalf("expected EXDEV from OpenNested, got %v", nestedErr)
	}

	root, err := Open(home, "account root")
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := root.WriteAtomic("welcome.txt", []byte("hi"), 0o640); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(backing, "welcome.txt"))
	if err != nil || string(got) != "hi" {
		t.Fatalf("write through bind mount: %v %q", err, got)
	}
}
