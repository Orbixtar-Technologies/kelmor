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
	"time"
)

func readyRetireJob(t *testing.T, st store.Store) {
	t.Helper()
	for _, job := range st.ListJobs("retrying", 20) {
		job.RunAfter = time.Now().Add(-time.Second)
		if err := st.UpdateJob(&job); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProvisionKeepsBootstrapPasswordUntilDependentsSucceed(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	acc := &store.Account{
		ID: "acc-boot", Username: "boot42", PrimaryDomain: "boot.test",
		PackageID: st.ListPackages()[0].ID, Status: "provisioning", HomePath: "/home/boot42",
		LinuxUID: 20130, LinuxGID: 20130, DesiredRevision: 1,
	}
	st.PutAccount(acc)
	st.PutDomain(&store.Domain{
		ID: "dom-boot", AccountID: acc.ID, FQDN: "boot.test", ASCII: "boot.test",
		Type: "primary", DocumentRoot: "/home/boot42/public_html", Status: "provisioning",
	})
	password := "KeepUntilDone!2026"
	queued, err := st.EnqueueJob(&store.Job{
		Type: "account.provision", ResourceType: "account", ResourceID: acc.ID,
		Payload: map[string]any{"account_id": acc.ID, "linux_password": password},
		State:   "queued", MaxAttempts: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	host := &operations.Host{Root: t.TempDir()}
	host.BeforeDispatch = func(_ context.Context, req operations.Request) error {
		if req.Method == "ApplyMailMaps" {
			return context.DeadlineExceeded
		}
		return nil
	}
	box, err := secret.FromBytes(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	New(st, host, logging.New("test"), box, "tester").Drain(context.Background())
	failed := st.GetJob(queued.ID)
	if failed == nil || failed.State != "retrying" {
		t.Fatalf("retry job: %+v", failed)
	}
	if failed.Payload["linux_password"] != password {
		t.Fatalf("password scrubbed before dependents succeeded: %+v", failed.Payload)
	}
}

func TestDKIMDoesNotEnableSigningWhenDNSPublishFails(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	acc := &store.Account{
		ID: "acc-dkim", Username: "dkim42", PrimaryDomain: "dkim.test",
		PackageID: st.ListPackages()[0].ID, Status: "active", HomePath: "/home/dkim42",
		LinuxUID: 20131, LinuxGID: 20131, DesiredRevision: 1,
	}
	st.PutAccount(acc)
	st.PutDomain(&store.Domain{
		ID: "dom-dkim", AccountID: acc.ID, FQDN: "dkim.test", ASCII: "dkim.test",
		Type: "primary", DocumentRoot: "/home/dkim42/public_html", Status: "active",
	})
	st.PutMailDomain(&store.MailDomain{ID: "md-dkim", AccountID: acc.ID, DomainID: "dom-dkim", Status: "active"})
	st.PutMailbox(&store.Mailbox{
		ID: "mb-dkim", AccountID: acc.ID, DomainID: "md-dkim",
		LocalPart: "info", PasswordHash: "hashed", Status: "active",
	})
	st.PutZone(&store.DNSZone{ID: "z-dkim", AccountID: acc.ID, DomainID: "dom-dkim", Name: "dkim.test", Provider: "powerdns"})
	root := t.TempDir()
	host := &operations.Host{Root: root}
	host.BeforeDispatch = func(_ context.Context, req operations.Request) error {
		if req.Method == "ApplyDNSZone" {
			return context.Canceled
		}
		return nil
	}
	w := New(st, host, logging.New("test"), nil, "tester")
	if err := w.applyMailStack(acc.ID); err == nil {
		t.Fatal("expected DNS publication failure")
	}
	if _, err := os.Stat(filepath.Join(root, "etc/rspamd/local.d/dkim_signing.conf")); !os.IsNotExist(err) {
		t.Fatal("DKIM signing applied after DNS publication failed")
	}
}

func TestApplicationRetireReachesTombstoneAfterCrashes(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	acc := &store.Account{
		ID: "acc-app", Username: "app42", PrimaryDomain: "app.test",
		PackageID: st.ListPackages()[0].ID, Status: "active", HomePath: "/home/app42",
		LinuxUID: 20132, LinuxGID: 20132, DesiredRevision: 1,
	}
	st.PutAccount(acc)
	app := &store.Application{
		ID: "app-1", AccountID: acc.ID, WebsiteID: "site-app",
		Runtime: "node", WorkingDirectory: "/home/app42/apps/site-app", Status: "running",
	}
	st.PutApp(app)
	root := t.TempDir()
	host := &operations.Host{Root: root}
	if _, err := host.CreateLinuxUser(acc.Username, acc.LinuxUID, acc.LinuxGID, acc.HomePath, "/usr/sbin/nologin"); err != nil {
		t.Fatal(err)
	}
	if _, err := host.Dispatch(context.Background(), operations.Request{
		Method: "ApplyAppUnit",
		Params: mustJSON(map[string]any{
			"website_id": app.WebsiteID, "account": acc.Username,
			"runtime": app.Runtime, "working_directory": app.WorkingDirectory,
		}),
	}); err != nil {
		t.Fatal(err)
	}

	_, err := st.EnqueueJob(&store.Job{
		Type: "application.retire", ResourceType: "application", ResourceID: app.ID,
		Payload: map[string]any{"application_id": app.ID, "account_id": acc.ID},
		State:   "queued",
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, stopAfter := range []string{"stop_unit", "remove_socket", "remove_unit", "remove_files"} {
		host.FailRetireAfter = stopAfter
		readyRetireJob(t, st)
		New(st, host, logging.New("test"), nil, "tester").Drain(context.Background())
		if st.GetApp(app.ID) == nil {
			t.Fatalf("row deleted before %s completed", stopAfter)
		}
	}
	host.FailRetireAfter = ""
	readyRetireJob(t, st)
	New(st, host, logging.New("test"), nil, "tester").Drain(context.Background())
	if st.GetApp(app.ID) != nil {
		t.Fatal("application desired-state row remained")
	}
	if _, err := os.Stat(filepath.Join(root, "etc/systemd/system/panel-app-site-app.service")); !os.IsNotExist(err) {
		t.Fatal("unit remained")
	}
	if _, err := os.Stat(filepath.Join(root, "run/panel/apps/site-app.sock")); !os.IsNotExist(err) {
		t.Fatal("socket remained")
	}
}
