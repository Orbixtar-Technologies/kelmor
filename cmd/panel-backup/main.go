package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("panel-backup create|restore|verify <account>")
		os.Exit(2)
	}
	fmt.Printf("backup engine %s — repositories: local, sftp, s3 (restic-compatible abstraction)\n", os.Args[1])
}
