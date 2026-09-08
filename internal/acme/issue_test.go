package acme

import (
	"context"
	"os"
	"testing"

	"github.com/hosting-panel/panel/agent/operations"
)

func TestIssueFallsBackToDevCert(t *testing.T) {
	h := &operations.Host{Root: t.TempDir()}
	if err := Issue(context.Background(), h, "acme.test", "ops@acme.test", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(h.Root + "/var/lib/panel/certs/acme.test.crt"); err != nil {
		t.Fatal(err)
	}
}
