package main

import (
	"crypto/ed25519"
	"fmt"
	"os"

	"github.com/hosting-panel/panel/internal/update"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Print(`panel-updater verify <manifest.json> <pubkey.hex|keyfile>

Sequence after verify: lock → download → hash files → backup control
database → install → migrate → switch current → restart → health-check.
Unsigned manifests are refused.
`)
		os.Exit(2)
	}
	if os.Args[1] != "verify" || len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: panel-updater verify <manifest.json> <pubkey>")
		os.Exit(2)
	}
	m, err := update.Load(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	raw, err := os.ReadFile(os.Args[3])
	if err != nil {
		raw = []byte(os.Args[3])
	}
	pub, err := update.ParsePublicKey(string(raw))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := update.Verify(m, ed25519.PublicKey(pub)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("verified release=%s channel=%s files=%d\n", m.Release, m.Channel, len(m.Files))
}
