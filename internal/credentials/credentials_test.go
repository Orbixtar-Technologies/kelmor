package credentials

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateRejectsKnownAdminPassword(t *testing.T) {
	root := t.TempDir()
	secrets, err := Generate(root)
	if err != nil {
		t.Fatal(err)
	}
	if secrets.AdminPassword == "" || secrets.AdminPassword == KnownAdminPassword {
		t.Fatal("generated administrator password must be unique")
	}
	if secrets.PowerDNSAPIKey == "" || secrets.PowerDNSAPIKey == KnownPowerDNSKey {
		t.Fatal("generated PowerDNS key must be unique")
	}
	info, err := os.Stat(filepath.Join(root, "var/lib/panel/secrets/admin-bootstrap"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("admin secret mode %o", info.Mode().Perm())
	}
}

func TestLoadReusesExistingSecrets(t *testing.T) {
	root := t.TempDir()
	first, err := Generate(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Generate(root)
	if err != nil {
		t.Fatal(err)
	}
	if first.AdminPassword != second.AdminPassword || first.PowerDNSAPIKey != second.PowerDNSAPIKey {
		t.Fatal("resume must preserve generated secrets")
	}
}

func TestRotatePowerDNSDoesNotReviveRetiredKey(t *testing.T) {
	root := t.TempDir()
	secrets, err := Generate(root)
	if err != nil {
		t.Fatal(err)
	}
	retired := secrets.PowerDNSAPIKey
	rotated, err := RotatePowerDNS(root)
	if err != nil {
		t.Fatal(err)
	}
	if rotated.PowerDNSAPIKey == retired {
		t.Fatal("rotation reused retired key")
	}
	if err := ActivatePowerDNS(root, rotated); err != nil {
		t.Fatal(err)
	}
	if err := RevokePowerDNS(root, retired); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.PowerDNSAPIKey != rotated.PowerDNSAPIKey {
		t.Fatal("active key not promoted")
	}
	if strings.Contains(string(mustRead(t, filepath.Join(root, "var/lib/panel/secrets/pdns-api-key"))), retired) {
		t.Fatal("retired key still present")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
