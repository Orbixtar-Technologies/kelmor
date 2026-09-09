package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/hosting-panel/panel/internal/update"
)

func TestParseUpdateConfigReadsPinnedPolicyAndInstalledRelease(t *testing.T) {
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

	config, err := parseUpdateConfigFile(configPath)
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

func TestParseUpdateConfigRejectsInvalidAutomaticSetting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update.env")
	if err := os.WriteFile(path, []byte("PANEL_UPDATE_AUTOMATIC=sometimes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := parseUpdateConfigFile(path); err == nil {
		t.Fatal("expected invalid automatic setting to be rejected")
	}
}

func TestLoadUpdateConfigRejectsCallerControlledPath(t *testing.T) {
	if _, err := loadUpdateConfig(filepath.Join(t.TempDir(), "update.env")); err == nil {
		t.Fatal("expected non-production config path to be rejected")
	}
}

func TestProductionConfigRequiresStableChannel(t *testing.T) {
	if err := validateProductionConfig(update.Config{Channel: "beta"}); err == nil {
		t.Fatal("expected non-stable production channel to be rejected")
	}
}

func TestRunWithAutomaticUpdatesDisabledDoesNotContactFeed(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer server.Close()

	status, err := executeRemoteCommand(context.Background(), "run", update.Config{
		FeedURL: server.URL, Channel: "stable", Automatic: false,
		InstallRoot: t.TempDir(),
	}, noOpRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "disabled" {
		t.Fatalf("disabled run status = %+v", status)
	}
	if hits.Load() != 0 {
		t.Fatalf("disabled run contacted feed %d times", hits.Load())
	}
}

type noOpRunner struct{}

func (noOpRunner) Run(context.Context, string, ...string) error {
	return nil
}
