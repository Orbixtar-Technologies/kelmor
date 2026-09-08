package update

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestApplyAndRollback(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	bundle := t.TempDir()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "panel-api"), []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	payload := []byte("new-bin")
	if err := os.WriteFile(filepath.Join(bundle, "panel-api"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	m := &Manifest{Release: "1.2.0", Channel: "stable", Files: map[string]string{"panel-api": hex.EncodeToString(sum[:])}}
	body, err := canonical(m)
	if err != nil {
		t.Fatal(err)
	}
	m.Signature = hex.EncodeToString(ed25519.Sign(priv, body))
	raw, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(bundle, "manifest.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Apply(bundle, root, pub); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "bin", "panel-api"))
	if string(got) != "new-bin" {
		t.Fatalf("apply wrote %q", got)
	}
	if err := Rollback(root); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(filepath.Join(root, "bin", "panel-api"))
	if string(got) != "old" {
		t.Fatalf("rollback wrote %q", got)
	}
}
