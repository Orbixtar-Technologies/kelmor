package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hosting-panel/panel/internal/update"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Print(`panel-updater <command>

  check --config /etc/panel/update.env
  install --config /etc/panel/update.env
  run --config /etc/panel/update.env
  status
  sign <bundle-dir> <release> <channel>
  verify <manifest.json> <pubkey.hex|keyfile>
  apply <bundle-dir> <install-root> <pubkey.hex|keyfile>
  rollback <install-root>

Apply verifies the signed manifest and file hashes, snapshots the
current binaries, then switches them. Rollback restores the snapshot.
Unsigned manifests are refused.
`)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "check", "install", "run":
		if len(os.Args) != 4 || os.Args[2] != "--config" {
			fatal("usage: panel-updater " + os.Args[1] + " --config <path>")
		}
		config, err := loadUpdateConfig(os.Args[3])
		if err != nil {
			fatal(err.Error())
		}
		status, err := executeRemoteCommand(context.Background(), os.Args[1], config, commandRunner{})
		if err != nil {
			fatal(err.Error())
		}
		writeJSON(status)
	case "status":
		if len(os.Args) != 2 {
			fatal("usage: panel-updater status")
		}
		raw, err := os.ReadFile(defaultStatusPath)
		if err != nil {
			fatal(err.Error())
		}
		fmt.Print(string(raw))
	case "sign":
		if len(os.Args) < 5 {
			fatal("usage: panel-updater sign <bundle-dir> <release> <channel>")
		}
		if err := signBundle(os.Args[2], os.Args[3], os.Args[4]); err != nil {
			fatal(err.Error())
		}
	case "verify":
		if len(os.Args) < 4 {
			fatal("usage: panel-updater verify <manifest.json> <pubkey>")
		}
		m, err := update.Load(os.Args[2])
		if err != nil {
			fatal(err.Error())
		}
		pub := mustPub(os.Args[3])
		if err := update.Verify(m, pub); err != nil {
			fatal(err.Error())
		}
		fmt.Printf("verified release=%s channel=%s files=%d\n", m.Release, m.Channel, len(m.Files))
	case "apply":
		if len(os.Args) < 5 {
			fatal("usage: panel-updater apply <bundle-dir> <install-root> <pubkey>")
		}
		if err := update.Apply(os.Args[2], os.Args[3], mustPub(os.Args[4])); err != nil {
			fatal(err.Error())
		}
		fmt.Println(`{"ok":true,"state":"applied"}`)
	case "rollback":
		if len(os.Args) < 3 {
			fatal("usage: panel-updater rollback <install-root>")
		}
		if err := update.Rollback(os.Args[2]); err != nil {
			fatal(err.Error())
		}
		fmt.Println(`{"ok":true,"state":"rolled-back"}`)
	default:
		fatal("unknown command")
	}
}

const (
	defaultInstallRoot   = "/usr/local/panel"
	defaultStatusPath    = "/var/lib/panel/update-status.json"
	defaultPublicKeyPath = "/etc/panel/update.pub"
)

type commandRunner struct{}

func (commandRunner) Run(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Run()
}

func executeRemoteCommand(
	ctx context.Context,
	command string,
	config update.Config,
	runner update.Runner,
) (*update.Status, error) {
	switch command {
	case "check":
		return update.Check(ctx, config)
	case "install":
		return update.Install(ctx, config, runner)
	case "run":
		if !config.Automatic {
			return &update.Status{
				State: "disabled", InstalledRelease: config.InstalledRelease,
				Automatic: false, Channel: config.Channel,
			}, nil
		}
		return update.Install(ctx, config, runner)
	default:
		return nil, fmt.Errorf("unsupported remote command %q", command)
	}
}

func loadUpdateConfig(path string) (update.Config, error) {
	if path != "/etc/panel/update.env" {
		return update.Config{}, fmt.Errorf("production config must be /etc/panel/update.env")
	}
	config, err := parseUpdateConfigFile(path)
	if err != nil {
		return update.Config{}, err
	}
	if err := validateProductionConfig(config); err != nil {
		return update.Config{}, err
	}
	return config, nil
}

func validateProductionConfig(config update.Config) error {
	if config.Channel != "stable" {
		return fmt.Errorf("production update channel must be stable")
	}
	return nil
}

func parseUpdateConfigFile(path string) (update.Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return update.Config{}, err
	}
	values := make(map[string]string)
	for lineNumber, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			return update.Config{}, fmt.Errorf("invalid config line %d", lineNumber+1)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 &&
			((value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		if _, duplicate := values[key]; duplicate {
			return update.Config{}, fmt.Errorf("duplicate config key %s", key)
		}
		values[key] = value
	}

	automatic := false
	if value, exists := values["PANEL_UPDATE_AUTOMATIC"]; exists {
		switch value {
		case "true":
			automatic = true
		case "false":
		default:
			return update.Config{}, fmt.Errorf("PANEL_UPDATE_AUTOMATIC must be true or false")
		}
	}
	installRoot := valueOrDefault(values["PANEL_UPDATE_INSTALL_ROOT"], defaultInstallRoot)
	statusPath := valueOrDefault(values["PANEL_UPDATE_STATUS_PATH"], defaultStatusPath)
	publicKeyPath := values["PANEL_UPDATE_PUBLIC_KEY"]
	if publicKeyPath == "" {
		publicKeyPath = valueOrDefault(values["PANEL_UPDATE_PUBLIC_KEY_PATH"], defaultPublicKeyPath)
	}
	publicKeyRaw, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return update.Config{}, fmt.Errorf("read pinned public key: %w", err)
	}
	publicKey, err := update.ParsePublicKey(string(publicKeyRaw))
	if err != nil {
		return update.Config{}, err
	}
	currentRelease, err := os.ReadFile(filepath.Join(installRoot, "current-release"))
	if err != nil {
		return update.Config{}, fmt.Errorf("read installed release: %w", err)
	}
	feedURL := values["PANEL_UPDATE_FEED_URL"]
	if feedURL == "" {
		return update.Config{}, fmt.Errorf("PANEL_UPDATE_FEED_URL is required")
	}
	channel := values["PANEL_UPDATE_CHANNEL"]
	if channel == "" {
		return update.Config{}, fmt.Errorf("PANEL_UPDATE_CHANNEL is required")
	}
	return update.Config{
		FeedURL: feedURL, Channel: channel,
		InstalledRelease: strings.TrimSpace(string(currentRelease)),
		PublicKey:        publicKey, InstallRoot: installRoot,
		StatusPath: statusPath, Automatic: automatic,
	}, nil
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func writeJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		fatal(err.Error())
	}
}

func signBundle(dir, release, channel string) error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	files := map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "manifest.json" || strings.HasPrefix(name, "release.") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		files[name] = hex.EncodeToString(sum[:])
	}
	if len(files) == 0 {
		return fmt.Errorf("no artifacts to sign")
	}
	m := &update.Manifest{Release: release, Channel: channel, Files: files}
	if err := update.Sign(m, priv); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "release.pub"), []byte(hex.EncodeToString(pub)+"\n"), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "release.priv"), []byte(hex.EncodeToString(priv)+"\n"), 0o600); err != nil {
		return err
	}
	fmt.Printf("signed release=%s files=%d\n", release, len(files))
	return nil
}

func mustPub(arg string) ed25519.PublicKey {
	raw, err := os.ReadFile(arg)
	if err != nil {
		raw = []byte(arg)
	}
	pub, err := update.ParsePublicKey(string(raw))
	if err != nil {
		fatal(err.Error())
	}
	return pub
}

func fatal(s string) {
	fmt.Fprintln(os.Stderr, s)
	os.Exit(1)
}
