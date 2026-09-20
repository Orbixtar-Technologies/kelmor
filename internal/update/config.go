package update

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultInstallRoot   = "/usr/local/panel"
	DefaultStatusPath    = "/var/lib/panel/update-status.json"
	DefaultPublicKeyPath = "/etc/panel/update.pub"
)

type ParsedConfig struct {
	FeedURL       string
	Channel       string
	Automatic     bool
	InstallRoot   string
	StatusPath    string
	PublicKeyPath string
}

func ParseConfigFile(path string) (ParsedConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ParsedConfig{}, err
	}
	values := make(map[string]string)
	for lineNumber, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			return ParsedConfig{}, fmt.Errorf("invalid config line %d", lineNumber+1)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 &&
			((value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		if _, duplicate := values[key]; duplicate {
			return ParsedConfig{}, fmt.Errorf("duplicate config key %s", key)
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
			return ParsedConfig{}, fmt.Errorf("PANEL_UPDATE_AUTOMATIC must be true or false")
		}
	}
	publicKeyPath := values["PANEL_UPDATE_PUBLIC_KEY"]
	if publicKeyPath == "" {
		publicKeyPath = valueOrDefault(values["PANEL_UPDATE_PUBLIC_KEY_PATH"], DefaultPublicKeyPath)
	}
	feedURL := values["PANEL_UPDATE_FEED_URL"]
	if feedURL == "" {
		return ParsedConfig{}, fmt.Errorf("PANEL_UPDATE_FEED_URL is required")
	}
	channel := values["PANEL_UPDATE_CHANNEL"]
	if channel == "" {
		return ParsedConfig{}, fmt.Errorf("PANEL_UPDATE_CHANNEL is required")
	}
	return ParsedConfig{
		FeedURL:       feedURL,
		Channel:       channel,
		Automatic:     automatic,
		InstallRoot:   valueOrDefault(values["PANEL_UPDATE_INSTALL_ROOT"], DefaultInstallRoot),
		StatusPath:    valueOrDefault(values["PANEL_UPDATE_STATUS_PATH"], DefaultStatusPath),
		PublicKeyPath: publicKeyPath,
	}, nil
}

func (parsed ParsedConfig) Materialize() (Config, error) {
	return parsed.MaterializeAt(parsed.PublicKeyPath, parsed.InstallRoot, parsed.StatusPath)
}

func (parsed ParsedConfig) MaterializeAt(publicKeyPath, installRoot, statusPath string) (Config, error) {
	publicKeyRaw, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return Config{}, fmt.Errorf("read pinned public key: %w", err)
	}
	publicKey, err := ParsePublicKey(string(publicKeyRaw))
	if err != nil {
		return Config{}, err
	}
	currentRelease, err := os.ReadFile(filepath.Join(installRoot, "current-release"))
	if err != nil {
		return Config{}, fmt.Errorf("read installed release: %w", err)
	}
	return Config{
		FeedURL:          parsed.FeedURL,
		Channel:          parsed.Channel,
		InstalledRelease: strings.TrimSpace(string(currentRelease)),
		PublicKey:        publicKey,
		InstallRoot:      installRoot,
		StatusPath:       statusPath,
		Automatic:        parsed.Automatic,
	}, nil
}

func LoadConfig(path string) (Config, error) {
	parsed, err := ParseConfigFile(path)
	if err != nil {
		return Config{}, err
	}
	return parsed.Materialize()
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
