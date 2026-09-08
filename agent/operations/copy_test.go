package operations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyHomedirCopiesPublicHTML(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	src := filepath.Join(root, "var/lib/panel/imports/acme42/homedir")
	if err := os.MkdirAll(filepath.Join(src, "public_html"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "public_html", "from-cpanel.html"), []byte("migrated"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := h.copyHomedir("acme42", "/var/lib/panel/imports/acme42/homedir", "/home/acme42")
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatal(res)
	}
	got, err := os.ReadFile(filepath.Join(root, "home/acme42/public_html/from-cpanel.html"))
	if err != nil || string(got) != "migrated" {
		t.Fatalf("copied %q %v", got, err)
	}
}

func TestCopyHomedirRejectsOutsideAccount(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if _, err := h.copyHomedir("acme42", "/var/lib/panel/imports/x", "/home/other"); err == nil {
		t.Fatal("expected dest boundary error")
	}
}
