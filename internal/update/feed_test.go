package update

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalManifestPathUsesInstallRootShareUpdates(t *testing.T) {
	got := LocalManifestPath("/usr/local/panel", "stable")
	want := "/usr/local/panel/share/updates/stable/manifest.json"
	if got != want {
		t.Fatalf("LocalManifestPath = %q, want %q", got, want)
	}
}

func TestWriteSignedCurrentReleaseFeedIsVerifiable(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := WriteSignedCurrentReleaseFeed(dir, "stable", "0.2.415", priv); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "stable", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Release != "0.2.415" || manifest.Channel != "stable" {
		t.Fatalf("manifest metadata: %+v", manifest)
	}
	if err := Verify(&manifest, pub); err != nil {
		t.Fatalf("seed feed signature: %v", err)
	}
}

func TestCopyFeedTreeInstallsChannelManifest(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "stable"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "stable", "manifest.json"), []byte(`{"release":"0.2.415"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CopyFeedTree(src, dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "stable", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"release":"0.2.415"}`+"\n" {
		t.Fatalf("copied manifest %q", got)
	}
}
