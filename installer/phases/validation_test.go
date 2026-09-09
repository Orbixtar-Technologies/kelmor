package phases

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplySMTPRelayWritesPostfixMaps(t *testing.T) {
	dir := t.TempDir()
	val := filepath.Join(dir, "validation")
	if err := os.MkdirAll(val, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(val, "smtp.env"), []byte("SMTP_HOST=mail.smtp2go.com\nSMTP_PORT=587\nSMTP_USER=orbixtar\nSMTP_SECRET_PATH=smtp_password\nSMTP_TLS_MODE=starttls\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(val, "smtp_password"), []byte("test-relay-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PANEL_VALIDATION_DIR", val)
	t.Setenv("PANEL_PUBLIC_IPV4", "203.0.113.10")

	root := filepath.Join(dir, "host")
	cfg := Config{Hostname: "lab.kelmor.host", Root: root}
	if err := applyMail(cfg); err != nil {
		t.Fatal(err)
	}
	mainb, err := os.ReadFile(filepath.Join(root, "etc/postfix/main.cf"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(mainb)
	if !contains(s, "relayhost = [mail.smtp2go.com]:587") {
		t.Fatalf("relayhost: %s", s)
	}
	if !contains(s, "smtp_sasl_auth_enable = yes") {
		t.Fatalf("sasl: %s", s)
	}
	sasl, err := os.ReadFile(filepath.Join(root, "etc/postfix/sasl_passwd"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(sasl), "orbixtar:test-relay-secret") {
		t.Fatalf("sasl_passwd: %s", sasl)
	}
	if err := verifyMail(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestWritePublicEnvIncludesValidationDomain(t *testing.T) {
	dir := t.TempDir()
	val := filepath.Join(dir, "validation")
	if err := os.MkdirAll(val, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(val, "domain.env"), []byte("TEST_DOMAIN=lab.kelmor.host\nNS1_HOSTNAME=ns1.kelmor.host\nNS2_HOSTNAME=ns2.kelmor.host\nVM_PUBLIC_IPV4=150.239.113.59\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PANEL_VALIDATION_DIR", val)
	t.Setenv("PANEL_PUBLIC_IPV4", "150.239.113.59")
	root := filepath.Join(dir, "host")
	cfg := Config{Hostname: "lab.kelmor.host", Root: root}
	if err := os.MkdirAll(filepath.Join(root, "var/lib/panel"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writePublicEnv(cfg); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "var/lib/panel/public.env"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !contains(s, "PANEL_PUBLIC_IPV4=150.239.113.59") {
		t.Fatalf("%s", s)
	}
	if !contains(s, "PANEL_TEST_DOMAIN=lab.kelmor.host") || !contains(s, "PANEL_NS1_HOSTNAME=ns1.kelmor.host") {
		t.Fatalf("%s", s)
	}
}

func TestValidationHostnameFromDomainEnv(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "domain.env"), []byte("TEST_DOMAIN=lab.kelmor.host\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PANEL_VALIDATION_DIR", dir)
	if got := ValidationHostname(); got != "lab.kelmor.host" {
		t.Fatalf("%q", got)
	}
}
