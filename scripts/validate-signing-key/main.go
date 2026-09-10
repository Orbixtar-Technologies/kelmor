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
	if *secret == pubHex {
		fatal(fmt.Errorf(
			"PANEL_UPDATE_SIGNING_KEY matches %s; store the private key (128 hex chars), not release.pub",
			*publicPath,
		))
	}

	priv, err := update.ParsePrivateKeyHex(*secret)
	if err != nil {
		fatal(fmt.Errorf("invalid PANEL_UPDATE_SIGNING_KEY: %w", err))
	}
	pub, err := update.ParsePublicKey(pubHex)
	if err != nil {
		fatal(fmt.Errorf("invalid public key in %s: %w", *publicPath, err))
	}
	if !update.PrivateKeyMatchesPublic(priv, pub) {
		fatal(fmt.Errorf(
			"PANEL_UPDATE_SIGNING_KEY does not match %s; regenerate a pair with scripts/ci/generate-release-signing-key.sh or restore the matching private key",
			*publicPath,
		))
	}

	normalized := strings.ToLower(strings.Map(func(r rune) rune {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
			return r
		default:
			return -1
		}
	}, *secret))
	if err := os.WriteFile(*outputPath, []byte(normalized+"\n"), 0o600); err != nil {
		fatal(fmt.Errorf("write signing key: %w", err))
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
