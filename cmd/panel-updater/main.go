package main

import (
	"crypto/ed25519"
	"fmt"
	"os"

	"github.com/hosting-panel/panel/internal/update"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Print(`panel-updater <command>

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
