package acme

import (
	"context"
	"os"
	"testing"

	"github.com/hosting-panel/panel/agent/operations"
)

func TestIssuerName(t *testing.T) {
	if IssuerName("") != "panel-dev" {
		t.Fatal(IssuerName(""))
	}
	if IssuerName("https://acme-staging-v02.api.letsencrypt.org/directory") != "letsencrypt-staging" {
		t.Fatal("staging")
	}
}

func TestIssueFallsBackToDevCert(t *testing.T) {
	h := &operations.Host{Root: t.TempDir()}
	if err := Issue(context.Background(), h, "acme.test", "ops@acme.test", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(h.Root + "/var/lib/panel/certs/acme.test.crt"); err != nil {
		t.Fatal(err)
	}
}
