package job

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/pkg/secret"
	"github.com/hosting-panel/panel/internal/store"
)

func TestProvisionWritesHostArtifacts(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	pkgs := st.ListPackages()
	acc := &store.Account{
		ID: "acc-1", Username: "acme42", PrimaryDomain: "acme.test",
		PackageID: pkgs[0].ID, Status: "provisioning", HomePath: "/home/acme42",
		LinuxUID: 20010, LinuxGID: 20010, DesiredRevision: 1,
	}
	st.PutAccount(acc)
	st.PutDomain(&store.Domain{
		ID: "dom-1", AccountID: acc.ID, FQDN: "acme.test", ASCII: "acme.test",
		Type: "primary", DocumentRoot: "/home/acme42/public_html", Status: "provisioning",
	})
	_, err := st.EnqueueJob(&store.Job{
		Type: "account.provision", ResourceType: "account", ResourceID: acc.ID,
		Payload: map[string]any{"account_id": acc.ID, "domain_id": "dom-1"}, State: "queued",
	})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	box, err := secret.FromBytes(bytes.Repeat([]byte{3}, 32))
	if err != nil {
		t.Fatal(err)
	}
	w := New(st, &operations.Host{Root: root}, logging.New("test"), box, "tester")
	w.Drain(context.Background())
	if got := st.GetAccount(acc.ID); got.Status != "active" {
		t.Fatalf("status %s", got.Status)
	}
	home := filepath.Join(root, "home", "acme42")
	if _, err := os.Stat(filepath.Join(home, "public_html")); err != nil {
		t.Fatal(err)
	}
	sites, err := os.ReadDir(filepath.Join(root, "etc/nginx/panel-sites"))
	if err != nil || len(sites) == 0 {
		t.Fatalf("nginx sites: %v %v", sites, err)
	}
	if _, err := os.Stat(filepath.Join(root, "var/lib/panel/mail/virtual")); err != nil {
		t.Fatal(err)
	}
	zones, err := os.ReadDir(filepath.Join(root, "var/lib/panel/dns/zones"))
	if err != nil || len(zones) == 0 {
		t.Fatalf("zones: %v %v", zones, err)
	}
	if _, err := os.Stat(filepath.Join(root, "var/lib/panel/certs/acme.test.crt")); err != nil {
		t.Fatal(err)
	}
	b := &store.BackupRun{ID: "bak-1", AccountID: acc.ID, Kind: "full", State: "queued", Destination: "local"}
	st.PutBackup(b)
	_, _ = st.EnqueueJob(&store.Job{Type: "backup.create", ResourceType: "backup", ResourceID: b.ID, Payload: map[string]any{"backup_id": b.ID}, State: "queued"})
	w.Drain(context.Background())
	got := st.GetBackup(b.ID)
	if got == nil || got.State != "succeeded" || got.Checksum == "" {
		t.Fatalf("backup %+v", got)
	}

	acc.Status = "terminating"
	acc.DesiredRevision++
	st.PutAccount(acc)
	_, _ = st.EnqueueJob(&store.Job{
		Type: "account.reconcile", ResourceType: "account", ResourceID: acc.ID,
		Payload: map[string]any{"account_id": acc.ID}, State: "queued",
	})
	w.Drain(context.Background())
	if got := st.GetAccount(acc.ID); got.Status != "terminated" {
		t.Fatalf("terminate status %s", got.Status)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("home remains after terminate: %v", err)
	}
	if entries, err := os.ReadDir(filepath.Join(root, "etc/nginx/panel-sites")); err == nil {
		for _, e := range entries {
			t.Fatalf("vhost remains after terminate: %s", e.Name())
		}
	}
}
