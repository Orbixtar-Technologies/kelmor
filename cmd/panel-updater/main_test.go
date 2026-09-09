package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUpdateConfigReadsPinnedPolicyAndInstalledRelease(t *testing.T) {
	root := t.TempDir()
	statusPath := filepath.Join(t.TempDir(), "status.json")
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	publicKeyPath := filepath.Join(t.TempDir(), "update.pub")
	if err := os.WriteFile(publicKeyPath, []byte(hex.EncodeToString(publicKey)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "current-release"), []byte("1.9.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "update.env")
	configBody := "PANEL_UPDATE_FEED_URL=https://updates.example.test/releases\n" +
		"PANEL_UPDATE_CHANNEL=stable\n" +
		"PANEL_UPDATE_AUTOMATIC=true\n" +
		"PANEL_UPDATE_PUBLIC_KEY=" + publicKeyPath + "\n" +
		"PANEL_UPDATE_INSTALL_ROOT=" + root + "\n" +
		"PANEL_UPDATE_STATUS_PATH=" + statusPath + "\n"
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := loadUpdateConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if config.FeedURL != "https://updates.example.test/releases" ||
		config.Channel != "stable" ||
		config.InstalledRelease != "1.9.0" ||
		config.InstallRoot != root ||
		config.StatusPath != statusPath ||
		!config.Automatic {
		t.Fatalf("unexpected config: %+v", config)
	}
	if !publicKey.Equal(config.PublicKey) {
		t.Fatal("configured pinned public key was not loaded")
	}
}

func TestLoadUpdateConfigRejectsInvalidAutomaticSetting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update.env")
	if err := os.WriteFile(path, []byte("PANEL_UPDATE_AUTOMATIC=sometimes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadUpdateConfig(path); err == nil {
		t.Fatal("expected invalid automatic setting to be rejected")
	}
}
