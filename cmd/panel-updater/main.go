package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Print(`panel-updater

Sequence: lock → download signed manifest → verify signature/hashes →
preflight → backup control database → install release → migrate →
switch current symlink → restart → health-check.

Unsigned downgrades are refused. Application code does not perform
unattended root-level OS upgrades.
`)
	if os.Getenv("PANEL_DEV") == "1" {
		fmt.Println("development channel: no remote manifest configured")
	}
}
