package operations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetLinuxPasswordRejectsUnsafe(t *testing.T) {
	h := &Host{Root: "/tmp/panel-linux-test"}
	if _, err := h.setLinuxPassword("acme42", "short"); err == nil {
		t.Fatal("short password")
	}
	if _, err := h.setLinuxPassword("acme42", "bad:colon!!"); err == nil {
		t.Fatal("colon")
	}
	res, err := h.setLinuxPassword("acme42", "SftpPass!2026")
	if err != nil || !res.OK {
		t.Fatalf("%v %#v", err, res)
	}
}

func TestHostingDirModeKeepsTenantsPrivate(t *testing.T) {
	if hostingDirMode("/home/acme/public_html") != 0o750 {
		t.Fatal("public_html must not be world-readable")
	}
	if hostingDirMode("/home/acme/backups") != 0o750 {
		t.Fatal("private dirs stay 0750")
	}
}

func TestCopyDirContentsPreservesFile(t *testing.T) {
	from := t.TempDir()
	to := t.TempDir()
	if err := os.WriteFile(filepath.Join(from, "note.txt"), []byte("hello"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := copyDirContents(from, to); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(to, "note.txt"))
	if err != nil || string(got) != "hello" {
		t.Fatalf("%v %q", err, got)
	}
}

func TestDirIsEmpty(t *testing.T) {
	empty, err := dirIsEmpty(t.TempDir())
	if err != nil || !empty {
		t.Fatalf("empty dir: %v %v", empty, err)
	}
}

func TestPlaceHomeSkipsWhenVolumeUnmounted(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if err := h.placeHomeOnQuotaVolume("acme42", "/home/acme42"); err != nil {
		t.Fatal(err)
	}
}
