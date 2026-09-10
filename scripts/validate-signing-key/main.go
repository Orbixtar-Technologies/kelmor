package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/hosting-panel/panel/internal/update"
)

func main() {
	secret := flag.String("secret", "", "raw signing secret from CI")
	publicPath := flag.String("public", "", "path to release.pub")
	outputPath := flag.String("output", "", "normalized private key file to write")
	flag.Parse()

	if *secret == "" || *publicPath == "" || *outputPath == "" {
		flag.Usage()
		os.Exit(2)
	}

	pubRaw, err := os.ReadFile(*publicPath)
	if err != nil {
		fatal(fmt.Errorf("read public key: %w", err))
	}
	pubHex := strings.TrimSpace(string(pubRaw))
	if strings.EqualFold(strings.TrimSpace(*secret), pubHex) {
		fatal(fmt.Errorf(
			"PANEL_UPDATE_SIGNING_KEY matches %s; store the private key, not release.pub",
			*publicPath,
		))
	}

	gotPub, err := update.PublicKeyHexFromMaterial(*secret)
	if err != nil {
		fatal(fmt.Errorf("invalid PANEL_UPDATE_SIGNING_KEY: %w", err))
	}
	if !strings.EqualFold(gotPub, pubHex) {
		fatal(fmt.Errorf(
			"PANEL_UPDATE_SIGNING_KEY public key %s does not match %s (%s); update the secret or commit the matching release.pub",
			gotPub, *publicPath, pubHex,
		))
	}

	normalized, err := update.NormalizeSigningSecret(*secret)
	if err != nil {
		fatal(fmt.Errorf("normalize private key: %w", err))
	}
	if err := os.WriteFile(*outputPath, normalized, 0o600); err != nil {
		fatal(fmt.Errorf("write signing key: %w", err))
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
