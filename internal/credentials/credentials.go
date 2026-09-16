package credentials

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	KnownAdminPassword = "ChangeMeOnce!2026"
	KnownPowerDNSKey   = "panel-loopback"

	adminSecretName = "admin-bootstrap"
	pdnsSecretName  = "pdns-api-key"
	retiredName     = "pdns-api-key.retired"
	preparedName    = "pdns-api-key.prepared"
)

type Secrets struct {
	AdminPassword  string
	PowerDNSAPIKey string
}

func secretDir(root string) string {
	return filepath.Join(root, "var", "lib", "panel", "secrets")
}

func Generate(root string) (Secrets, error) {
	if existing, err := Load(root); err == nil {
		return existing, nil
	}
	admin, err := randomSecret(24)
	if err != nil {
		return Secrets{}, err
	}
	pdns, err := randomSecret(24)
	if err != nil {
		return Secrets{}, err
	}
	if admin == KnownAdminPassword || pdns == KnownPowerDNSKey {
		return Secrets{}, fmt.Errorf("generated a repository-known secret")
	}
	if err := os.MkdirAll(secretDir(root), 0o750); err != nil {
		return Secrets{}, err
	}
	if err := writeSecret(filepath.Join(secretDir(root), adminSecretName), admin); err != nil {
		return Secrets{}, err
	}
	if err := writeSecret(filepath.Join(secretDir(root), pdnsSecretName), pdns); err != nil {
		return Secrets{}, err
	}
	return Secrets{AdminPassword: admin, PowerDNSAPIKey: pdns}, nil
}

func Load(root string) (Secrets, error) {
	admin, err := readSecret(filepath.Join(secretDir(root), adminSecretName))
	if err != nil {
		return Secrets{}, err
	}
	pdns, err := readSecret(filepath.Join(secretDir(root), pdnsSecretName))
	if err != nil {
		return Secrets{}, err
	}
	return Secrets{AdminPassword: admin, PowerDNSAPIKey: pdns}, nil
}

func LoadRuntime() (Secrets, error) {
	if dir := strings.TrimSpace(os.Getenv("CREDENTIALS_DIRECTORY")); dir != "" {
		admin, err := readSecret(filepath.Join(dir, adminSecretName))
		if err == nil {
			pdns, pdnsErr := readSecret(filepath.Join(dir, pdnsSecretName))
			if pdnsErr == nil {
				return Secrets{AdminPassword: admin, PowerDNSAPIKey: pdns}, nil
			}
		}
	}
	if root := strings.TrimSpace(os.Getenv("PANEL_STATE_DIR")); root != "" {
		if secrets, err := Load(filepath.Join(root, "host")); err == nil {
			return secrets, nil
		}
		if secrets, err := Load("/"); err == nil && fileExists(filepath.Join("/var/lib/panel/secrets", adminSecretName)) {
			return secrets, nil
		}
	}
	if fileExists("/var/lib/panel/secrets/" + adminSecretName) {
		return Load("/")
	}
	if os.Getenv("PANEL_DEV") == "1" {
		admin := os.Getenv("PANEL_ADMIN_PASSWORD")
		if admin == "" {
			admin = KnownAdminPassword
		}
		pdns := os.Getenv("PANEL_PDNS_API_KEY")
		if pdns == "" {
			pdns = KnownPowerDNSKey
		}
		return Secrets{AdminPassword: admin, PowerDNSAPIKey: pdns}, nil
	}
	return Secrets{}, fmt.Errorf("administrator credentials are not installed")
}

func RotatePowerDNS(root string) (Secrets, error) {
	current, err := Load(root)
	if err != nil {
		return Secrets{}, err
	}
	next, err := randomSecret(24)
	if err != nil {
		return Secrets{}, err
	}
	if next == current.PowerDNSAPIKey || next == KnownPowerDNSKey {
		return Secrets{}, fmt.Errorf("rotation reused a retired or known key")
	}
	if err := writeSecret(filepath.Join(secretDir(root), preparedName), next); err != nil {
		return Secrets{}, err
	}
	return Secrets{AdminPassword: current.AdminPassword, PowerDNSAPIKey: next}, nil
}

func ActivatePowerDNS(root string, secrets Secrets) error {
	if secrets.PowerDNSAPIKey == "" {
		return fmt.Errorf("missing prepared PowerDNS key")
	}
	return writeSecret(filepath.Join(secretDir(root), pdnsSecretName), secrets.PowerDNSAPIKey)
}

func RevokePowerDNS(root string, retired string) error {
	if retired == "" {
		return nil
	}
	current, err := Load(root)
	if err != nil {
		return err
	}
	if current.PowerDNSAPIKey == retired {
		return fmt.Errorf("refusing to revoke the active PowerDNS key")
	}
	_ = os.Remove(filepath.Join(secretDir(root), preparedName))
	return writeSecret(filepath.Join(secretDir(root), retiredName), retired)
}

func writeSecret(path, value string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(value+"\n"), 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func readSecret(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(raw))
	if value == "" {
		return "", fmt.Errorf("empty secret file")
	}
	return value, nil
}

func randomSecret(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
