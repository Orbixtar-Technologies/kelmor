package update

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"testing"
)

func TestParsePrivateKeyHexAcceptsFullPrivateKey(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParsePrivateKeyHex(hex.EncodeToString(priv))
	if err != nil {
		t.Fatal(err)
	}
	if !PrivateKeyMatchesPublic(got, priv.Public().(ed25519.PublicKey)) {
		t.Fatal("parsed private key mismatch")
	}
}

func TestParsePrivateKeyHexAcceptsSeed(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	seed := priv.Seed()
	got, err := ParsePrivateKeyHex(hex.EncodeToString(seed))
	if err != nil {
		t.Fatal(err)
	}
	if !PrivateKeyMatchesPublic(got, priv.Public().(ed25519.PublicKey)) {
		t.Fatal("seed-derived private key mismatch")
	}
}

func TestParsePrivateKeyHexIgnoresWhitespace(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	raw := hex.EncodeToString(priv)
	spaced := strings.Join([]string{raw[:32], raw[32:64], raw[64:]}, "\n")
	if _, err := ParsePrivateKeyHex(spaced); err != nil {
		t.Fatal(err)
	}
}

func TestParsePrivateKeyHexRejectsInvalidHex(t *testing.T) {
	if _, err := ParsePrivateKeyHex("not-a-private-key"); err == nil {
		t.Fatal("expected invalid hex to be rejected")
	}
}
