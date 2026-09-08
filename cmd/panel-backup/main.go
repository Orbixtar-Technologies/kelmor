package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hosting-panel/panel/internal/backup"
	"github.com/hosting-panel/panel/internal/pkg/secret"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("panel-backup verify <repo-root> <key> [checksum]")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "verify":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "verify <repo-root> <object-key> [checksum]")
			os.Exit(2)
		}
		repo := &backup.Local{Root: os.Args[2]}
		sum := ""
		if len(os.Args) > 4 {
			sum = os.Args[4]
		}
		if err := repo.Verify(context.Background(), os.Args[3], sum); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("ok")
	case "keyfp":
		box, err := secret.LoadOrCreate(os.Getenv("PANEL_MASTER_KEY"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(box.Fingerprint())
	default:
		fmt.Fprintf(os.Stderr, "unknown command %s — repositories: local, sftp, s3\n", os.Args[1])
		os.Exit(2)
	}
}
