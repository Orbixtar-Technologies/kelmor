package operations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMeasureAccountUsageWalksSandboxHome(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home", "acme42")
	if err := os.MkdirAll(filepath.Join(home, "public_html"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "public_html", "index.html"), []byte("hello-usage"), 0o640); err != nil {
		t.Fatal(err)
	}
	h := &Host{Root: root}
	u, err := h.measureAccountUsage("acme42", "/home/acme42")
	if err != nil {
		t.Fatal(err)
	}
	if u.DiskBytes < 11 || u.InodeCount < 2 {
		t.Fatalf("%+v", u)
	}
}
