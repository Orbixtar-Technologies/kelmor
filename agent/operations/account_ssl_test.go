package operations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	paneltls "github.com/hosting-panel/panel/internal/tls"
)

func TestInstallAccountCertificateWritesFullchain(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	cert, key, err := paneltls.SelfSigned("shop.test", time.Now().Add(40*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	ca, _, err := paneltls.SelfSigned("Kelmor Intermediate", time.Now().Add(400*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := h.installAccountCertificate("shop.test", string(cert), string(key), string(ca))
	if err != nil {
		t.Fatal(err)
	}
	result, _ := raw.(map[string]any)
	if result["issuer"] == "" || result["hostname"] != "shop.test" {
		t.Fatalf("result %#v", result)
	}
	written, err := os.ReadFile(filepath.Join(h.Root, "var/lib/panel/certs/shop.test.crt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "BEGIN CERTIFICATE") {
		t.Fatal("certificate PEM missing")
	}
	if strings.Count(string(written), "BEGIN CERTIFICATE") < 2 {
		t.Fatal("CA bundle must be appended to the site certificate file")
	}
	if _, err := os.Stat(filepath.Join(h.Root, "var/lib/panel/certs/shop.test.key")); err != nil {
		t.Fatal(err)
	}
}

func TestInstallAccountCertificateRejectsInvalidMaterial(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	cert, key, err := paneltls.SelfSigned("shop.test", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	_, other, err := paneltls.SelfSigned("other.test", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.installAccountCertificate("shop.test", "not-a-pem", string(key), ""); err == nil {
		t.Fatal("garbage PEM must fail")
	}
	if _, err := h.installAccountCertificate("shop.test", string(cert), string(other), ""); err == nil {
		t.Fatal("mismatched key must fail")
	}
	if _, err := h.installAccountCertificate("shop.test", string(cert), string(key), "-----BEGIN CERTIFICATE-----\nbad\n-----END CERTIFICATE-----\n"); err == nil {
		t.Fatal("invalid CA bundle must fail")
	}
	if _, err := os.Stat(filepath.Join(h.Root, "var/lib/panel/certs/shop.test.crt")); err == nil {
		t.Fatal("failed install must not invent a certificate file")
	}
}

func TestInstallAccountCertificateRequiresHostnameCoverage(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	cert, key, err := paneltls.SelfSigned("other.test", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.installAccountCertificate("shop.test", string(cert), string(key), ""); err == nil {
		t.Fatal("cert that does not cover the hostname must fail")
	}
}
