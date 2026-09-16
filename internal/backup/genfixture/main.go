package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hosting-panel/panel/internal/backup"
	"github.com/hosting-panel/panel/internal/pkg/secret"
	"github.com/hosting-panel/panel/internal/store"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: genfixture <dst.hpm>")
		os.Exit(2)
	}
	root, err := os.MkdirTemp("", "hpm1-golden-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(root)
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(filepath.Join(home, "public_html"), 0o750); err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(home, "public_html", "index.html"), []byte("golden-hpm1-site"), 0o644); err != nil {
		panic(err)
	}
	raw, err := backup.PackHome(home)
	if err != nil {
		panic(err)
	}
	box, err := secret.FromBytes(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		panic(err)
	}
	repo := &backup.Local{Root: filepath.Join(root, "repo")}
	acc := &store.Account{ID: "a-golden", Username: "acme42", HomePath: "/home/acme42"}
	_, key, err := backup.BuildArchive(context.Background(), box, repo, acc, nil, nil, raw)
	if err != nil {
		panic(err)
	}
	src := filepath.Join(root, "repo", key)
	body, err := os.ReadFile(src)
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll(filepath.Dir(os.Args[1]), 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(os.Args[1], body, 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("wrote %s (%d bytes, key %s)\n", os.Args[1], len(body), key)
}
