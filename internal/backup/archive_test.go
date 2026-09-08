package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hosting-panel/panel/internal/pkg/secret"
	"github.com/hosting-panel/panel/internal/store"
)

func TestBuildRestoreRoundTrip(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home", "acme42")
	if err := os.MkdirAll(filepath.Join(home, "public_html"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "public_html", "index.html"), []byte("hello-site"), 0o644); err != nil {
		t.Fatal(err)
	}
	key := bytes.Repeat([]byte{7}, 32)
	box, err := secret.FromBytes(key)
	if err != nil {
		t.Fatal(err)
	}
	repo := &Local{Root: filepath.Join(root, "repo")}
	acc := &store.Account{ID: "a1", Username: "acme42", HomePath: "/home/acme42"}
	man, obj, err := Build(context.Background(), box, repo, acc, nil, nil, home)
	if err != nil {
		t.Fatal(err)
	}
	if man.Checksums["files.tar.gz"] == "" || obj == "" {
		t.Fatal(man)
	}
	dest := filepath.Join(root, "restore")
	got, err := Restore(context.Background(), box, repo, obj, dest)
	if err != nil {
		t.Fatal(err)
	}
	if err := Preflight(got, acc); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dest, "public_html", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "hello-site" {
		t.Fatalf("got %q", body)
	}
}

func TestRestoreRejectsTraversal(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: "../escape.txt", Mode: 0o644, Size: 3}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("no\n")); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := unpackHome(buf.Bytes(), t.TempDir()); err == nil {
		t.Fatal("expected traversal error")
	}
}
