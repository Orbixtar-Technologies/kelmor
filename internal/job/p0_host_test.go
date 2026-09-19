package job

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/pkg/secret"
	"github.com/hosting-panel/panel/internal/store"
)

func TestProvisionAppliesDedicatedIPAndShellClass(t *testing.T) {
	t.Setenv("PANEL_PUBLIC_IPV4", "203.0.113.10")
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	pkgs := st.ListPackages()
	acc := &store.Account{
		ID: "acc-ip", Username: "ipuser1", PrimaryDomain: "ip.test",
		PackageID: pkgs[0].ID, Status: "provisioning", HomePath: "/home/ipuser1",
		LinuxUID: 20040, LinuxGID: 20040, DesiredRevision: 1,
		IPAddress: "198.51.100.77", ShellClass: "jailed",
	}
	st.PutAccount(acc)
	st.PutDomain(&store.Domain{
		ID: "dom-ip", AccountID: acc.ID, FQDN: "ip.test", ASCII: "ip.test",
		Type: "primary", DocumentRoot: "/home/ipuser1/public_html", Status: "provisioning",
	})
	_, err := st.EnqueueJob(&store.Job{
		Type: "account.provision", ResourceType: "account", ResourceID: acc.ID,
		Payload: map[string]any{"account_id": acc.ID, "domain_id": "dom-ip"},
		State:   "queued",
	})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	box, err := secret.FromBytes(bytes.Repeat([]byte{5}, 32))
	if err != nil {
		t.Fatal(err)
	}
	w := New(st, &operations.Host{Root: root}, logging.New("test"), box, "tester")
	w.Drain(context.Background())
	identity, err := os.ReadFile(filepath.Join(root, "home/ipuser1/.panel-identity"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(identity, []byte("shell=/usr/sbin/rssh")) {
		t.Fatalf("jailed shell not applied: %s", identity)
	}
	zone := st.ZoneByDomain("dom-ip")
	if zone == nil {
		t.Fatal("missing zone")
	}
	haveDedicatedA := false
	for _, rec := range st.ListRecords(zone.ID) {
		if rec.Type == "A" && rec.Name == "@" && rec.Content == "198.51.100.77" {
			haveDedicatedA = true
		}
	}
	if !haveDedicatedA {
		t.Fatal("dedicated IPv4 was not published on the apex A record")
	}
	sites, err := os.ReadDir(filepath.Join(root, "etc/nginx/panel-sites"))
	if err != nil {
		t.Fatal(err)
	}
	bound := false
	for _, e := range sites {
		body, err := os.ReadFile(filepath.Join(root, "etc/nginx/panel-sites", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "listen 198.51.100.77:80") {
			bound = true
			break
		}
	}
	if !bound {
		t.Fatal("vhost did not bind the assigned public IPv4")
	}
}

func TestResetAccountBandwidthClearsHoldWithoutUnsuspending(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	acc := &store.Account{
		ID: "acc-hold", Username: "hold42", PrimaryDomain: "hold.test",
		PackageID: st.ListPackages()[0].ID, Status: "suspended", HomePath: "/home/hold42",
		LinuxUID: 20050, LinuxGID: 20050,
	}
	st.PutAccount(acc)
	st.PutUsage(&store.Usage{AccountID: acc.ID, BandwidthBytes: 9 << 30, BandwidthHold: true})
	root := t.TempDir()
	box, err := secret.FromBytes(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	w := New(st, &operations.Host{Root: root}, logging.New("test"), box, "tester")
	if err := w.resetAccountBandwidth(&store.Job{Payload: map[string]any{"account_id": acc.ID}}); err != nil {
		t.Fatal(err)
	}
	if got := st.GetAccount(acc.ID); got.Status != "suspended" {
		t.Fatalf("status changed to %s", got.Status)
	}
	u := st.GetUsage(acc.ID)
	if u == nil || u.BandwidthHold || u.BandwidthBytes != 0 {
		t.Fatalf("hold not cleared: %+v", u)
	}
	flag, err := os.ReadFile(filepath.Join(root, "var/lib/panel/bandwidth/hold42/hold"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(flag)) != "0" {
		t.Fatalf("host hold flag %q", flag)
	}
}
