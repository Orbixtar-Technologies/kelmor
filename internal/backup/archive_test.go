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
	if man.FormatVersion != 2 {
		t.Fatalf("format %d", man.FormatVersion)
	}
}

func TestPackSplitV2RoundTrip(t *testing.T) {
	home, err := PackHome(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := PackV2(home, []DBDump{{Engine: "mariadb", Name: "acme_site", SQL: []byte("SELECT 1;\n")}}, []MailDump{{Domain: "acme.test", Local: "info", TarGz: []byte("not-a-real-tar")}})
	if err != nil {
		t.Fatal(err)
	}
	gotHome, dbs, mail, err := SplitV2(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotHome) == 0 || len(dbs) != 1 || dbs[0].Name != "acme_site" || string(dbs[0].SQL) != "SELECT 1;\n" {
		t.Fatalf("%+v", dbs)
	}
	if len(mail) != 1 || mail[0].Domain != "acme.test" || mail[0].Local != "info" {
		t.Fatalf("%+v", mail)
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
