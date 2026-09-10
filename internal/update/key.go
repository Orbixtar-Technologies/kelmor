package update

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

// ParsePrivateKeyMaterial loads an Ed25519 private key from raw hex (seed or
// full private key bytes). OpenSSH PEM secrets should use ParseSigningSigner.
func ParsePrivateKeyMaterial(raw string) (ed25519.PrivateKey, error) {
	pemBytes, err := decodeSigningSecret(raw)
	if err != nil {
		return nil, err
	}
	if strings.Contains(string(pemBytes), "BEGIN") {
		return nil, fmt.Errorf("OpenSSH private key must be used via ParseSigningSigner")
	}
	return parseRawEd25519PrivateKey(pemBytes)
}

// ParsePrivateKeyHex loads an Ed25519 private key from hex text. It accepts a
// 64-byte private key or a 32-byte seed.
func ParsePrivateKeyHex(raw string) (ed25519.PrivateKey, error) {
	return ParsePrivateKeyMaterial(raw)
}

// ParseSigningSigner loads an Ed25519 signer from hex text, PEM, or hex-encoded
// PEM.
func ParseSigningSigner(raw string) (ssh.Signer, error) {
	pemBytes, err := decodeSigningSecret(raw)
	if err != nil {
		return nil, err
	}
	if strings.Contains(string(pemBytes), "BEGIN") {
		signer, err := ssh.ParsePrivateKey(pemBytes)
		if err != nil {
			return nil, fmt.Errorf("parse private key PEM: %w", err)
		}
		if signer.PublicKey().Type() != ssh.KeyAlgoED25519 {
			return nil, fmt.Errorf("private key must be ed25519, got %s", signer.PublicKey().Type())
		}
		return signer, nil
	}
	priv, err := parseRawEd25519PrivateKey(pemBytes)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return nil, fmt.Errorf("build signer from ed25519 key: %w", err)
	}
	return signer, nil
}

// PublicKeyHexFromMaterial returns the lowercase hex public key for raw secret
// material.
func PublicKeyHexFromMaterial(raw string) (string, error) {
	signer, err := ParseSigningSigner(raw)
	if err != nil {
		return "", err
	}
	m := signer.PublicKey().Marshal()
	if len(m) < ed25519.PublicKeySize {
		return "", fmt.Errorf("unexpected ssh public key encoding")
	}
	return hex.EncodeToString(m[len(m)-ed25519.PublicKeySize:]), nil
}

// NormalizeSigningSecret writes key bytes suitable for a -priv file: raw hex
// for native ed25519 keys, or PEM bytes for OpenSSH keys.
func NormalizeSigningSecret(raw string) ([]byte, error) {
	pemBytes, err := decodeSigningSecret(raw)
	if err != nil {
		return nil, err
	}
	if strings.Contains(string(pemBytes), "BEGIN") {
		return pemBytes, nil
	}
	priv, err := parseRawEd25519PrivateKey(pemBytes)
	if err != nil {
		return nil, err
	}
	return []byte(hex.EncodeToString(priv) + "\n"), nil
}

// PrivateKeyMatchesPublic reports whether priv corresponds to pub.
func PrivateKeyMatchesPublic(priv ed25519.PrivateKey, pub ed25519.PublicKey) bool {
	return string(priv.Public().(ed25519.PublicKey)) == string(pub)
}

func decodeSigningSecret(raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("private key is empty")
	}
	if strings.Contains(trimmed, "BEGIN") {
		return []byte(trimmed), nil
	}
	hexKey := extractHex(trimmed)
	if hexKey == "" {
		return nil, fmt.Errorf("private key must be hex or PEM")
	}
	return decodeFlexibleHex(hexKey)
}

func parseRawEd25519PrivateKey(privBytes []byte) (ed25519.PrivateKey, error) {
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

func extractHex(raw string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
			return r
		default:
			return -1
		}
	}, raw)
}

func decodeFlexibleHex(hexKey string) ([]byte, error) {
	if len(hexKey)%2 == 1 {
		hexKey = hexKey[:len(hexKey)-1]
	}
	return hex.DecodeString(hexKey)
}
