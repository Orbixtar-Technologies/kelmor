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
	certs := st.ListCerts(acc.ID)
	if len(certs) != 1 || certs[0].Hostname != "acme.test" || certs[0].Issuer != "panel-dev" || certs[0].Status != "active" {
		t.Fatalf("certificate %+v", certs)
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

func TestReconcileRewritesLoopbackARecords(t *testing.T) {
	t.Setenv("PANEL_PUBLIC_IPV4", "203.0.113.50")
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	pkgs := st.ListPackages()
	acc := &store.Account{
		ID: "acc-dns", Username: "dns42", PrimaryDomain: "oldip.test",
		PackageID: pkgs[0].ID, Status: "active", HomePath: "/home/dns42",
		LinuxUID: 20040, LinuxGID: 20040, DesiredRevision: 2,
	}
	st.PutAccount(acc)
	st.PutDomain(&store.Domain{
		ID: "dom-dns", AccountID: acc.ID, FQDN: "oldip.test", ASCII: "oldip.test",
		Type: "primary", DocumentRoot: "/home/dns42/public_html", Status: "active",
	})
	st.PutZone(&store.DNSZone{ID: "z-dns", AccountID: acc.ID, DomainID: "dom-dns", Name: "oldip.test", Provider: "powerdns", DesiredRevision: 1})
	st.PutRecord(&store.DNSRecord{ID: "r-a", ZoneID: "z-dns", Name: "@", Type: "A", Content: "127.0.0.1", TTL: 3600})
	st.PutRecord(&store.DNSRecord{ID: "r-spf", ZoneID: "z-dns", Name: "@", Type: "TXT", Content: "v=spf1 a mx ip4:127.0.0.1 ~all", TTL: 3600})
	st.PutRecord(&store.DNSRecord{ID: "r-custom", ZoneID: "z-dns", Name: "cdn", Type: "A", Content: "198.51.100.9", TTL: 3600})
	root := t.TempDir()
	box, err := secret.FromBytes(bytes.Repeat([]byte{3}, 32))
	if err != nil {
		t.Fatal(err)
	}
	w := New(st, &operations.Host{Root: root}, logging.New("test"), box, "tester")
	if err := w.provisionAccount(&store.Job{
		Type: "account.reconcile", ResourceType: "account", ResourceID: acc.ID,
		Payload: map[string]any{"account_id": acc.ID},
	}); err != nil {
		t.Fatal(err)
	}
	var apex, spf, custom string
	for _, rec := range st.ListRecords("z-dns") {
		switch {
		case rec.Type == "A" && rec.Name == "@":
			apex = rec.Content
		case rec.Type == "TXT" && rec.Name == "@":
			spf = rec.Content
		case rec.Type == "A" && rec.Name == "cdn":
			custom = rec.Content
		}
	}
	if apex != "203.0.113.50" {
		t.Fatalf("apex A %q", apex)
	}
	if !bytes.Contains([]byte(spf), []byte("ip4:203.0.113.50")) || bytes.Contains([]byte(spf), []byte("ip4:127.0.0.1")) {
		t.Fatalf("spf %q", spf)
	}
	if custom != "198.51.100.9" {
		t.Fatalf("custom A overwritten: %q", custom)
	}
}

func TestRestoreDoesNotUnsuspend(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	pkgs := st.ListPackages()
	acc := &store.Account{
		ID: "acc-rs", Username: "rs42", PrimaryDomain: "rs.test",
		PackageID: pkgs[0].ID, Status: "provisioning", HomePath: "/home/rs42",
		LinuxUID: 20013, LinuxGID: 20013, DesiredRevision: 1,
	}
	st.PutAccount(acc)
	st.PutDomain(&store.Domain{
		ID: "dom-rs", AccountID: acc.ID, FQDN: "rs.test", ASCII: "rs.test",
		Type: "primary", DocumentRoot: "/home/rs42/public_html", Status: "provisioning",
	})
	root := t.TempDir()
	box, err := secret.FromBytes(bytes.Repeat([]byte{3}, 32))
	if err != nil {
		t.Fatal(err)
	}
	w := New(st, &operations.Host{Root: root}, logging.New("test"), box, "tester")
	if err := w.provisionAccount(&store.Job{
		Type: "account.provision", ResourceType: "account", ResourceID: acc.ID,
		Payload: map[string]any{"account_id": acc.ID, "domain_id": "dom-rs"},
	}); err != nil {
		t.Fatal(err)
	}
	b := &store.BackupRun{ID: "bak-rs", AccountID: acc.ID, Kind: "full", State: "queued", Destination: "local"}
	st.PutBackup(b)
	if err := w.createBackup(&store.Job{Payload: map[string]any{"backup_id": b.ID}}); err != nil {
		t.Fatal(err)
	}
	acc = st.GetAccount(acc.ID)
	acc.Status = "suspended"
	acc.DesiredRevision++
	st.PutAccount(acc)
	_ = filepath.Walk(filepath.Join(root, "home", "rs42"), func(p string, _ os.FileInfo, err error) error {
		if err == nil {
			_ = os.Chmod(p, 0o755)
		}
		return nil
	})
	if err := w.restoreBackup(&store.Job{Payload: map[string]any{"backup_id": b.ID, "account_id": acc.ID}}); err != nil {
		t.Fatal(err)
	}
	if got := st.GetAccount(acc.ID); got.Status != "suspended" {
		t.Fatalf("restore unsuspended account: %s", got.Status)
	}
}

func TestReconcileKeepsLaterSuspend(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	pkgs := st.ListPackages()
	acc := &store.Account{
		ID: "acc-hold", Username: "hold42", PrimaryDomain: "hold.test",
		PackageID: pkgs[0].ID, Status: "suspended", HomePath: "/home/hold42",
		LinuxUID: 20012, LinuxGID: 20012, DesiredRevision: 4,
	}
	st.PutAccount(acc)
	st.PutDomain(&store.Domain{
		ID: "dom-hold", AccountID: acc.ID, FQDN: "hold.test", ASCII: "hold.test",
		Type: "primary", DocumentRoot: "/home/hold42/public_html", Status: "active",
	})
	st.PutWebsite(&store.Website{
		ID: "web-hold", AccountID: acc.ID, DomainID: "dom-hold", Runtime: "php",
		DocumentRoot: "/home/hold42/public_html", Enabled: true,
	})
	root := t.TempDir()
	box, err := secret.FromBytes(bytes.Repeat([]byte{3}, 32))
	if err != nil {
		t.Fatal(err)
	}
	w := New(st, &operations.Host{Root: root}, logging.New("test"), box, "tester")
	if err := w.provisionAccount(&store.Job{
		Type: "account.reconcile", ResourceType: "account", ResourceID: acc.ID,
		Payload: map[string]any{"account_id": acc.ID},
	}); err != nil {
		t.Fatal(err)
	}
	if got := st.GetAccount(acc.ID); got.Status != "suspended" {
		t.Fatalf("reconcile cleared suspend: %s", got.Status)
	}
	conf, err := os.ReadFile(filepath.Join(root, "etc/nginx/panel-sites/web-hold.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(conf, []byte("account suspended")) {
		t.Fatalf("suspended vhost: %s", conf)
	}
}

func TestReconcileKeepsLaterUnsuspend(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	pkgs := st.ListPackages()
	acc := &store.Account{
		ID: "acc-race", Username: "race42", PrimaryDomain: "race.test",
		PackageID: pkgs[0].ID, Status: "active", HomePath: "/home/race42",
		LinuxUID: 20011, LinuxGID: 20011, DesiredRevision: 3,
	}
	st.PutAccount(acc)
	st.PutDomain(&store.Domain{
		ID: "dom-race", AccountID: acc.ID, FQDN: "race.test", ASCII: "race.test",
		Type: "primary", DocumentRoot: "/home/race42/public_html", Status: "active",
	})
	root := t.TempDir()
	box, err := secret.FromBytes(bytes.Repeat([]byte{3}, 32))
	if err != nil {
		t.Fatal(err)
	}
	w := New(st, &operations.Host{Root: root}, logging.New("test"), box, "tester")
	if err := w.provisionAccount(&store.Job{
		Type: "account.reconcile", ResourceType: "account", ResourceID: acc.ID,
		Payload: map[string]any{"account_id": acc.ID, "status": "suspended"},
	}); err != nil {
		t.Fatal(err)
	}
	if got := st.GetAccount(acc.ID); got.Status != "active" {
		t.Fatalf("stale suspend payload overwrote status: %s", got.Status)
	}
}

func TestHostedDatabaseReusesSharedPassword(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	acc := &store.Account{
		ID: "acc-db", Username: "dbshare", PrimaryDomain: "dbshare.test",
		PackageID: st.ListPackages()[0].ID, Status: "active", HomePath: "/home/dbshare",
		LinuxUID: 20100, LinuxGID: 20100,
	}
	st.PutAccount(acc)
	st.PutDB(&store.HostedDatabase{ID: "db-1", AccountID: acc.ID, Engine: "mariadb", Name: "dbshare_one", Status: "queued"})
	st.PutDB(&store.HostedDatabase{ID: "db-2", AccountID: acc.ID, Engine: "mariadb", Name: "dbshare_two", Status: "queued"})
	root := t.TempDir()
	box, err := secret.FromBytes(bytes.Repeat([]byte{3}, 32))
	if err != nil {
		t.Fatal(err)
	}
	w := New(st, &operations.Host{Root: root}, logging.New("test"), box, "tester")
	if err := w.provisionDB(&store.Job{Payload: map[string]any{"database_id": "db-1"}}); err != nil {
		t.Fatal(err)
	}
	if err := w.provisionDB(&store.Job{Payload: map[string]any{"database_id": "db-2"}}); err != nil {
		t.Fatal(err)
	}
	users := st.ListDBUsers(acc.ID)
	if len(users) != 1 || len(users[0].PasswordEnc) == 0 {
		t.Fatalf("shared db user %+v", users)
	}
	one, err := os.ReadFile(filepath.Join(root, "home/dbshare/.panel-database.mariadb.dbshare_one"))
	if err != nil {
		t.Fatal(err)
	}
	two, err := os.ReadFile(filepath.Join(root, "home/dbshare/.panel-database.mariadb.dbshare_two"))
	if err != nil {
		t.Fatal(err)
	}
	pass := bytes.SplitN(bytes.SplitN(one, []byte("password="), 2)[1], []byte("\n"), 2)[0]
	if len(pass) == 0 || !bytes.Contains(two, pass) {
		t.Fatalf("passwords diverged\n%s\n%s", one, two)
	}
}

func TestHostedDBCredentialsKeepEnginesSeparate(t *testing.T) {
	st := store.NewMemory()
	box, err := secret.FromBytes(bytes.Repeat([]byte{3}, 32))
	if err != nil {
		t.Fatal(err)
	}
	w := New(st, &operations.Host{Root: t.TempDir()}, logging.New("test"), box, "tester")
	acc := &store.Account{ID: "acc-eng", Username: "acme42"}
	u1, p1, reset1 := w.hostedDBCredentials(acc, "mariadb")
	u2, p2, reset2 := w.hostedDBCredentials(acc, "postgres")
	if u1 != "acme42_u" || u1 != u2 || p1 == "" || p1 == p2 || !reset1 || !reset2 {
		t.Fatalf("first pair %s %s %v / %s %s %v", u1, p1, reset1, u2, p2, reset2)
	}
	_, again, resetAgain := w.hostedDBCredentials(acc, "mariadb")
	if again != p1 || resetAgain {
		t.Fatalf("mariadb password drifted %s %v", again, resetAgain)
	}
	users := st.ListDBUsers(acc.ID)
	if len(users) != 2 {
		t.Fatalf("expected per-engine users, got %+v", users)
	}
}
