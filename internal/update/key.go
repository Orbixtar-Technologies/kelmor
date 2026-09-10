package update

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"
)

// ParsePrivateKeyHex loads an Ed25519 private key from hex text. It accepts a
// 64-byte private key or a 32-byte seed.
func ParsePrivateKeyHex(raw string) (ed25519.PrivateKey, error) {
	hexKey := strings.Map(func(r rune) rune {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
			return r
		default:
			return -1
		}
	}, raw)
	if hexKey == "" {
		return nil, fmt.Errorf("private key must be hexadecimal")
	}
	privBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("private key is not valid hex: %w", err)
	}
	switch len(privBytes) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(privBytes), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(privBytes), nil
	default:
		return nil, fmt.Errorf(
			"private key must be %d hex chars (%d-byte seed) or %d hex chars (%d-byte private key); got %d bytes after decode",
			ed25519.SeedSize*2, ed25519.SeedSize,
			ed25519.PrivateKeySize*2, ed25519.PrivateKeySize,
			len(privBytes),
		)
	}
}

// PrivateKeyMatchesPublic reports whether priv corresponds to pub.
func PrivateKeyMatchesPublic(priv ed25519.PrivateKey, pub ed25519.PublicKey) bool {
	return string(priv.Public().(ed25519.PublicKey)) == string(pub)
}
