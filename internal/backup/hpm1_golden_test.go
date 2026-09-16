package backup

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hosting-panel/panel/internal/pkg/secret"
	"github.com/hosting-panel/panel/internal/store"
)

func TestHPM1GoldenFixtureRestoresUnchanged(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "hpm1-acme42.hpm"))
	if err != nil {
		t.Fatal(err)
	}
	box, err := secret.FromBytes(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	repo := &Local{Root: t.TempDir()}
	const objectKey = "acme42/golden.hpm"
	if err := repo.Put(context.Background(), objectKey, raw); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	man, err := Restore(context.Background(), box, repo, objectKey, dest)
	if err != nil {
		t.Fatal(err)
	}
	if man.FormatVersion != 1 {
		t.Fatalf("format %d", man.FormatVersion)
	}
	if man.Username != "acme42" {
		t.Fatalf("username %q", man.Username)
	}
	body, err := os.ReadFile(filepath.Join(dest, "public_html", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "golden-hpm1-site" {
		t.Fatalf("got %q", body)
	}
	acc := &store.Account{Username: "acme42"}
	if err := Preflight(man, acc); err != nil {
		t.Fatal(err)
	}
}
