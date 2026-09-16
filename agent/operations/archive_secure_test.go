package operations

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestUnpackDirectoryRejectsTraversalAndLinks(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "home", "acme42", "restore")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "var", "tmp", "panel-imports", "evil.tgz")
	if err := os.MkdirAll(filepath.Dir(archive), 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	writeTarHeader(t, tw, &tar.Header{Name: "../etc/passwd", Mode: 0644, Size: 4, Typeflag: tar.TypeReg})
	if _, err := tw.Write([]byte("pwn\n")); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &Host{Root: root}
	if _, err := h.unpackDirectory("/var/tmp/panel-imports/evil.tgz", "/home/acme42/restore"); err == nil {
		t.Fatal("expected archive traversal to fail")
	}
}

func TestUnpackDirectoryRejectsDuplicateAndDevice(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "home", "acme42", "restore"), 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "var", "tmp", "panel-imports", "dup.tgz")
	if err := os.MkdirAll(filepath.Dir(archive), 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	writeTarHeader(t, tw, &tar.Header{Name: "a.txt", Mode: 0644, Size: 3, Typeflag: tar.TypeReg})
	if _, err := tw.Write([]byte("one")); err != nil {
		t.Fatal(err)
	}
	writeTarHeader(t, tw, &tar.Header{Name: "a.txt", Mode: 0644, Size: 3, Typeflag: tar.TypeReg})
	if _, err := tw.Write([]byte("two")); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &Host{Root: root}
	if _, err := h.unpackDirectory("/var/tmp/panel-imports/dup.tgz", "/home/acme42/restore"); err == nil {
		t.Fatal("expected duplicate archive entry to fail")
	}
}

func writeTarHeader(t *testing.T, tw *tar.Writer, hdr *tar.Header) {
	t.Helper()
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
}
