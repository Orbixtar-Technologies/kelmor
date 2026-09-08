package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hosting-panel/panel/internal/update"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Print(`panel-updater <command>

  sign <bundle-dir> <release> <channel>
  verify <manifest.json> <pubkey.hex|keyfile>
  apply <bundle-dir> <install-root> <pubkey.hex|keyfile>
  rollback <install-root>

Apply verifies the signed manifest and file hashes, snapshots the
current binaries, then switches them. Rollback restores the snapshot.
Unsigned manifests are refused.
`)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "sign":
		if len(os.Args) < 5 {
			fatal("usage: panel-updater sign <bundle-dir> <release> <channel>")
		}
		if err := signBundle(os.Args[2], os.Args[3], os.Args[4]); err != nil {
			fatal(err.Error())
		}
	case "verify":
		if len(os.Args) < 4 {
			fatal("usage: panel-updater verify <manifest.json> <pubkey>")
		}
		m, err := update.Load(os.Args[2])
		if err != nil {
			fatal(err.Error())
		}
		pub := mustPub(os.Args[3])
		if err := update.Verify(m, pub); err != nil {
			fatal(err.Error())
		}
		fmt.Printf("verified release=%s channel=%s files=%d\n", m.Release, m.Channel, len(m.Files))
	case "apply":
		if len(os.Args) < 5 {
			fatal("usage: panel-updater apply <bundle-dir> <install-root> <pubkey>")
		}
		if err := update.Apply(os.Args[2], os.Args[3], mustPub(os.Args[4])); err != nil {
			fatal(err.Error())
		}
		fmt.Println(`{"ok":true,"state":"applied"}`)
	case "rollback":
		if len(os.Args) < 3 {
			fatal("usage: panel-updater rollback <install-root>")
		}
		if err := update.Rollback(os.Args[2]); err != nil {
			fatal(err.Error())
		}
		fmt.Println(`{"ok":true,"state":"rolled-back"}`)
	default:
		fatal("unknown command")
	}
}

func signBundle(dir, release, channel string) error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	files := map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "manifest.json" || strings.HasPrefix(name, "release.") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		files[name] = hex.EncodeToString(sum[:])
	}
	if len(files) == 0 {
		return fmt.Errorf("no artifacts to sign")
	}
	m := &update.Manifest{Release: release, Channel: channel, Files: files}
	if err := update.Sign(m, priv); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "release.pub"), []byte(hex.EncodeToString(pub)+"\n"), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "release.priv"), []byte(hex.EncodeToString(priv)+"\n"), 0o600); err != nil {
		return err
	}
	fmt.Printf("signed release=%s files=%d\n", release, len(files))
	return nil
}

func mustPub(arg string) ed25519.PublicKey {
	raw, err := os.ReadFile(arg)
	if err != nil {
		raw = []byte(arg)
	}
	pub, err := update.ParsePublicKey(string(raw))
	if err != nil {
		fatal(err.Error())
	}
	return pub
}

func fatal(s string) {
	fmt.Fprintln(os.Stderr, s)
	os.Exit(1)
}
