package operations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadDeleteRenameManagedFile(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home", "acme42", "public_html")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "note.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &Host{Root: root}
	read, err := h.readManagedFile("/home/acme42/public_html/note.txt")
	if err != nil {
		t.Fatal(err)
	}
	if read.Message != "aGVsbG8=" {
		t.Fatalf("content: %q", read.Message)
	}
	if _, err := h.deleteManagedFile("/home/acme42/public_html/note.txt"); err != nil {
		t.Fatal(err)
	}
}
