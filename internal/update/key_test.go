package update

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/pem"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
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
	got, err := ParsePrivateKeyHex(hex.EncodeToString(priv.Seed()))
	if err != nil {
		t.Fatal(err)
	}
	if !PrivateKeyMatchesPublic(got, priv.Public().(ed25519.PublicKey)) {
		t.Fatal("seed-derived private key mismatch")
	}
}

func TestParseSigningSignerAcceptsOpenSSHPEM(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(block)
	signer, err := ParseSigningSigner(string(pemBytes))
	if err != nil {
		t.Fatal(err)
	}
	gotPub, err := PublicKeyHexFromMaterial(string(pemBytes))
	if err != nil {
		t.Fatal(err)
	}
	want := hex.EncodeToString(priv.Public().(ed25519.PublicKey))
	if gotPub != want {
		t.Fatalf("public key mismatch: %s != %s", gotPub, want)
	}
	if signer.PublicKey().Type() != ssh.KeyAlgoED25519 {
		t.Fatalf("unexpected key type %s", signer.PublicKey().Type())
	}
}

func TestParseSigningSignerAcceptsHexEncodedOpenSSHPEM(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	encoded := hex.EncodeToString(pem.EncodeToMemory(block))
	if _, err := ParseSigningSigner(encoded); err != nil {
		t.Fatal(err)
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
