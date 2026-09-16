package phases

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var allowedInstallKeys = map[string]struct{}{
	"hostname":        {},
	"admin_email":     {},
	"admin_password":  {},
	"channel":         {},
	"acme":            {},
	"non_interactive": {},
	"dev":             {},
	"root":            {},
}

func DiscoverInstallFile(explicit, exeDir, cwd string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	for _, dir := range []string{cwd, exeDir} {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		candidate := filepath.Join(dir, "install.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func LoadInstallFile(path string) (Config, error) {
	var cfg Config
	body, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	for i, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return Config{}, fmt.Errorf("%s:%d: expected key: value", path, i+1)
		}
		key = strings.TrimSpace(key)
		value = unquoteYAML(strings.TrimSpace(value))
		if _, known := allowedInstallKeys[key]; !known {
			return Config{}, fmt.Errorf("%s:%d: unknown key %q", path, i+1, key)
		}
		switch key {
		case "hostname":
			cfg.Hostname = value
		case "admin_email":
			cfg.AdminEmail = value
		case "admin_password":
			cfg.AdminPassword = value
		case "channel":
			cfg.Channel = value
		case "acme":
			cfg.ACMEMode = value
		case "root":
			cfg.Root = value
		case "non_interactive":
			flag, err := parseYAMLBool(value)
			if err != nil {
				return Config{}, fmt.Errorf("%s:%d: %w", path, i+1, err)
			}
			cfg.NonInteractive = flag
		case "dev":
			flag, err := parseYAMLBool(value)
			if err != nil {
				return Config{}, fmt.Errorf("%s:%d: %w", path, i+1, err)
			}
			cfg.Dev = flag
		default:
			return Config{}, fmt.Errorf("%s:%d: unhandled key %q", path, i+1, key)
		}
	}
	return cfg, nil
}

func MergeInstallConfig(file, cli Config, setFlags map[string]bool) Config {
	out := file
	if setFlags["hostname"] {
		out.Hostname = cli.Hostname
	}
	if setFlags["admin-email"] {
		out.AdminEmail = cli.AdminEmail
	}
	if setFlags["admin-password"] {
		out.AdminPassword = cli.AdminPassword
	} else if out.AdminPassword == "" {
		out.AdminPassword = cli.AdminPassword
	}
	if setFlags["channel"] {
		out.Channel = cli.Channel
	}
	if setFlags["acme"] {
		out.ACMEMode = cli.ACMEMode
	}
	if setFlags["root"] {
		out.Root = cli.Root
	} else if out.Root == "" {
		out.Root = cli.Root
	}
	if setFlags["config"] {
		out.ConfigPath = cli.ConfigPath
	}
	if setFlags["non-interactive"] || cli.NonInteractive {
		out.NonInteractive = true
	}
	if setFlags["dev"] || cli.Dev {
		out.Dev = true
	}
	if out.Channel == "" {
		out.Channel = "stable"
	}
	return out
}

func PromptMissing(cfg *Config, in io.Reader, out io.Writer) error {
	if cfg.NonInteractive {
		return nil
	}
	reader := bufio.NewReader(in)
	hostname, err := promptLine(reader, out, "Hostname", cfg.Hostname)
	if err != nil {
		return err
	}
	if hostname != "" {
		cfg.Hostname = hostname
	}
	adminDefault := cfg.AdminEmail
	if adminDefault == "" && cfg.Hostname != "" {
		adminDefault = "admin@" + cfg.Hostname
	}
	admin, err := promptLine(reader, out, "Administrator email", adminDefault)
	if err != nil {
		return err
	}
	if admin != "" {
		cfg.AdminEmail = admin
	}
	password, err := promptLine(reader, out, "Administrator password (empty generates one)", cfg.AdminPassword)
	if err != nil {
		return err
	}
	cfg.AdminPassword = password
	acmeDefault := cfg.ACMEMode
	if acmeDefault == "" {
		acmeDefault = "letsencrypt"
	}
	acme, err := promptLine(reader, out, "ACME directory (letsencrypt, staging, pebble)", acmeDefault)
	if err != nil {
		return err
	}
	if acme != "" {
		cfg.ACMEMode = acme
	}
	return nil
}

func promptLine(reader *bufio.Reader, out io.Writer, label, fallback string) (string, error) {
	if fallback != "" {
		fmt.Fprintf(out, "%s [%s]: ", label, fallback)
	} else {
		fmt.Fprintf(out, "%s: ", label)
	}
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return fallback, nil
	}
	return line, nil
}

func unquoteYAML(value string) string {
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func parseYAMLBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "yes", "1":
		return true, nil
	case "false", "no", "0", "":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean %q", value)
	}
}
