package acme

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hosting-panel/panel/agent/operations"
)

func TestDirectoryReadsFile(t *testing.T) {
	if Directory() != os.Getenv("PANEL_ACME_DIRECTORY") && os.Getenv("PANEL_ACME_DIRECTORY") != "" {
		t.Fatal("env wins")
	}
	if !insecureDirectory("https://127.0.0.1:14000/dir") {
		t.Fatal("lab ACME must skip TLS verify")
	}
	if insecureDirectory("https://acme-v02.api.letsencrypt.org/directory") {
		t.Fatal("public LE must verify TLS")
	}
}

func TestIssuerName(t *testing.T) {
	if IssuerName("") != "panel-dev" {
		t.Fatal(IssuerName(""))
	}
	if IssuerName("https://acme-staging-v02.api.letsencrypt.org/directory") != "letsencrypt-staging" {
		t.Fatal("staging")
	}
	if IssuerName("https://127.0.0.1:14000/dir") != "pebble" {
		t.Fatal("local pebble")
	}
}

func TestIssueFallsBackToDevCert(t *testing.T) {
	h := &operations.Host{Root: t.TempDir()}
	exp, err := Issue(context.Background(), h, "acme.test", "ops@acme.test", "")
	if err != nil {
		t.Fatal(err)
	}
	if exp.Before(time.Now().Add(80 * 24 * time.Hour)) {
		t.Fatalf("expected ~90d notAfter, got %s", exp)
	}
	if _, err := os.Stat(h.Root + "/var/lib/panel/certs/acme.test.crt"); err != nil {
		t.Fatal(err)
	}
}
