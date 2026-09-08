package phases

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrefixPreflightRequiresNoble(t *testing.T) {
	root := t.TempDir()
	cfg := Config{Hostname: "panel.example.net", Root: root}
	if err := checkPreflight(cfg); err == nil {
		t.Fatal("expected missing os-release")
	}
	if err := os.MkdirAll(filepath.Join(root, "etc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "etc/os-release"), []byte("NAME=\"Ubuntu\"\nVERSION_ID=\"24.04\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkPreflight(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestUseLabACMEModes(t *testing.T) {
	t.Setenv("PANEL_ACME_DIRECTORY", "")
	t.Setenv("PANEL_ACME_LAB", "")
	if !useLabACME(Config{Dev: true}) {
		t.Fatal("dev is lab")
	}
	if !useLabACME(Config{ACMEMode: "pebble"}) {
		t.Fatal("pebble mode")
	}
	if useLabACME(Config{ACMEMode: "letsencrypt"}) {
		t.Fatal("letsencrypt mode is production")
	}
	t.Setenv("PANEL_ACME_LAB", "1")
	if !useLabACME(Config{}) {
		t.Fatal("PANEL_ACME_LAB")
	}
	t.Setenv("PANEL_ACME_LAB", "")
	t.Setenv("PANEL_ACME_DIRECTORY", pebbleDirectory)
	if !useLabACME(Config{}) {
		t.Fatal("env pebble")
	}
	t.Setenv("PANEL_ACME_DIRECTORY", letsEncryptProd)
	if useLabACME(Config{}) {
		t.Fatal("env letsencrypt is production")
	}
}

func TestWritePublicACMEUsesLetsEncrypt(t *testing.T) {
	t.Setenv("PANEL_ACME_DIRECTORY", "")
	t.Setenv("PANEL_ACME_LAB", "")
	t.Setenv("PANEL_INSTALL_ROOT", "")
	root := t.TempDir()
	cfg := Config{Hostname: "panel.example.net", ACMEMode: "letsencrypt", Root: root}
	if err := applyTLS(cfg); err != nil {
		t.Fatal(err)
	}
	if err := verifyTLS(cfg); err != nil {
		t.Fatal(err)
	}
	dir := readACMEDirectory(cfg)
	if dir != letsEncryptProd {
		t.Fatalf("directory %s", dir)
	}
	env, err := os.ReadFile(filepath.Join(root, "var/lib/panel/acme.env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(env), letsEncryptProd) {
		t.Fatalf("acme.env %s", env)
	}
	if strings.Contains(string(env), "PANEL_ACME_INSECURE=1") {
		t.Fatal("public LE must verify TLS")
	}
}

func TestWritePublicACMEStaging(t *testing.T) {
	t.Setenv("PANEL_ACME_DIRECTORY", "")
	t.Setenv("PANEL_ACME_STAGING", "")
	root := t.TempDir()
	cfg := Config{Hostname: "panel.example.net", ACMEMode: "staging", Root: root}
	if err := writePublicACME(cfg); err != nil {
		t.Fatal(err)
	}
	if readACMEDirectory(cfg) != letsEncryptStaging {
		t.Fatal(readACMEDirectory(cfg))
	}
}

func TestLabACMEKeepsExistingPebbleDirectory(t *testing.T) {
	t.Setenv("PANEL_ACME_DIRECTORY", "")
	t.Setenv("PANEL_ACME_LAB", "")
	root := t.TempDir()
	cfg := Config{Hostname: "panel.example.net", Root: root}
	if err := os.MkdirAll(filepath.Join(root, "var/lib/panel"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "var/lib/panel/acme.directory"), []byte(pebbleDirectory+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !useLabACME(cfg) {
		t.Fatal("existing pebble directory must stay lab")
	}
}

func TestFreshPrefixDefaultsToLetsEncrypt(t *testing.T) {
	t.Setenv("PANEL_ACME_DIRECTORY", "")
	t.Setenv("PANEL_ACME_LAB", "")
	t.Setenv("PANEL_INSTALL_ROOT", "")
	root := t.TempDir()
	cfg := Config{Hostname: "panel.example.net", Root: root}
	if useLabACME(cfg) {
		t.Fatal("empty prefix install must default to Let's Encrypt")
	}
	if err := ensureACME(cfg); err != nil {
		t.Fatal(err)
	}
	if readACMEDirectory(cfg) != letsEncryptProd {
		t.Fatal(readACMEDirectory(cfg))
	}
}
