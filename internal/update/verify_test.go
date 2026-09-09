package update

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyRejectsUnsigned(t *testing.T) {
	m := &Manifest{Release: "1.0.0", Channel: "stable", Signature: "unsigned-development"}
	if err := Verify(m, make(ed25519.PublicKey, ed25519.PublicKeySize)); err == nil {
		t.Fatal("expected unsigned reject")
	}
}

func TestVerifyRoundTrip(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	m := &Manifest{Release: "1.0.0", Channel: "stable", Files: map[string]string{"panel-api": "abc"}}
	if err := Sign(m, priv); err != nil {
		t.Fatal(err)
	}
	if err := Verify(m, pub); err != nil {
		t.Fatal(err)
	}
}

func TestManifestSignatureCoversCompatibilityAndArtifacts(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	m := &Manifest{
		Release: "2.0.0", Channel: "stable", MinimumRelease: "1.0.0",
		Artifacts: []Artifact{{
			Path: "panel-api", Target: "bin/panel-api", Size: 9,
			SHA256: strings.Repeat("a", 64), Mode: 0o755,
		}},
	}
	if err := Sign(m, priv); err != nil {
		t.Fatal(err)
	}
	if err := Verify(m, pub); err != nil {
		t.Fatalf("signed manifest rejected: %v", err)
	}

	m.MinimumRelease = "1.5.0"
	if err := Verify(m, pub); err == nil {
		t.Fatal("signature did not cover minimum compatible release")
	}
	m.MinimumRelease = "1.0.0"
	m.Artifacts[0].Target = "bin/panel-worker"
	if err := Verify(m, pub); err == nil {
		t.Fatal("signature did not cover artifact target")
	}
}

func TestVerifyFileHashes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.bin"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &Manifest{Files: map[string]string{"a.bin": "8f434346648f6b96df89dda901c5176b10a6d83961dd3c1ac88b59b2dc327aa4"}}
	// sha256("hi")
	if err := VerifyFileHashes(m, dir); err != nil {
		t.Fatal(err)
	}
}
