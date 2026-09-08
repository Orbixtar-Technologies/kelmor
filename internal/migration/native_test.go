package migration

import (
	"encoding/json"
	"testing"

	"github.com/hosting-panel/panel/internal/store"
)

func TestExportImportRoundTrip(t *testing.T) {
	st := store.NewMemory()
	acc := &store.Account{ID: "acc1", Username: "acme42", PrimaryDomain: "acme.test", HomePath: "/home/acme42", Status: "active", LinuxUID: 20001, LinuxGID: 20001}
	st.PutAccount(acc)
	st.PutDomain(&store.Domain{ID: "d1", AccountID: acc.ID, FQDN: "acme.test", ASCII: "acme.test", Type: "primary"})
	exp, err := Export(st, acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(exp)
	if err != nil {
		t.Fatal(err)
	}
	dst := store.NewMemory()
	if err := store.SeedDev(dst, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	owner := dst.UserByUsername("admin")
	got, err := ImportAs(dst, raw, "", "", owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "acme42" {
		t.Fatal(got)
	}
	if dst.AccountByUsername("acme42") == nil {
		t.Fatal("missing imported account")
	}
	jobs := dst.ListJobs("queued", 10)
	if len(jobs) == 0 {
		t.Fatal("expected reconcile job")
	}
}

func TestImportAsRenames(t *testing.T) {
	src := store.NewMemory()
	src.PutAccount(&store.Account{ID: "acc1", Username: "acme42", PrimaryDomain: "acme.test", HomePath: "/home/acme42", Status: "active", LinuxUID: 20001, LinuxGID: 20001})
	src.PutDomain(&store.Domain{ID: "d1", AccountID: "acc1", FQDN: "acme.test", ASCII: "acme.test", Type: "primary", DocumentRoot: "/home/acme42/public_html"})
	src.PutDomain(&store.Domain{ID: "d2", AccountID: "acc1", FQDN: "blog.acme.test", ASCII: "blog.acme.test", Type: "addon", DocumentRoot: "/home/acme42/blog.acme.test/public_html"})
	src.PutWebsite(&store.Website{ID: "w1", AccountID: "acc1", DomainID: "d1", Runtime: "php", DocumentRoot: "/home/acme42/public_html"})
	src.PutWebsite(&store.Website{ID: "w2", AccountID: "acc1", DomainID: "d2", Runtime: "php", DocumentRoot: "/home/acme42/blog.acme.test/public_html"})
	src.PutMailDomain(&store.MailDomain{ID: "md1", AccountID: "acc1", DomainID: "d1", CatchallPolicy: "reject", Status: "active"})
	src.PutMailbox(&store.Mailbox{ID: "mb1", AccountID: "acc1", DomainID: "md1", LocalPart: "info", PasswordHash: "hash-info", Status: "active"})
	src.PutMailAlias(&store.MailAlias{ID: "al1", AccountID: "acc1", DomainID: "md1", Address: "sales", Destination: "info@acme.test"})
	src.PutDB(&store.HostedDatabase{ID: "db1", AccountID: "acc1", Engine: "mariadb", Name: "acme42_app", Status: "active"})
	src.PutCron(&store.CronJob{ID: "cr1", AccountID: "acc1", Schedule: "0 * * * *", Command: "php cron.php", WorkingDirectory: "/home/acme42/public_html", Enabled: true})
	src.PutFTP(&store.FTPAccount{ID: "ftp1", AccountID: "acc1", Username: "acme42_ftp", HomePath: "/home/acme42/public_html", PasswordHash: "hash-ftp", Status: "active"})
	exp, err := Export(src, "acc1")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(exp)
	if err := store.SeedDev(src, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	got, err := ImportAs(src, raw, "moved42", "moved.test", src.UserByUsername("admin").ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "moved42" || got.PrimaryDomain != "moved.test" || got.ID == "acc1" {
		t.Fatalf("%+v", got)
	}
	if src.AccountByUsername("acme42") == nil || src.AccountByUsername("moved42") == nil {
		t.Fatal("both accounts should exist")
	}
	sites := src.ListWebsites(got.ID)
	if len(sites) < 2 {
		t.Fatalf("expected remapped websites: %+v", sites)
	}
	roots := map[string]bool{}
	for _, s := range sites {
		roots[s.DocumentRoot] = true
	}
	if !roots["/home/moved42/public_html"] || !roots["/home/moved42/blog.moved.test/public_html"] {
		t.Fatalf("document roots not remapped: %+v", sites)
	}
	var addon store.Domain
	for _, d := range src.ListDomains(got.ID) {
		if d.Type == "addon" {
			addon = d
		}
	}
	if addon.ASCII != "blog.moved.test" {
		t.Fatalf("addon domain: %+v", addon)
	}
	dbs := src.ListDBs(got.ID)
	if len(dbs) != 1 || dbs[0].Name != "moved42_app" {
		t.Fatalf("db rename: %+v", dbs)
	}
	crons := src.ListCrons(got.ID)
	if len(crons) != 1 || crons[0].WorkingDirectory != "/home/moved42/public_html" {
		t.Fatalf("cron: %+v", crons)
	}
	aliases := src.ListMailAliases(got.ID)
	if len(aliases) != 1 || aliases[0].Destination != "info@moved.test" {
		t.Fatalf("alias: %+v", aliases)
	}
	boxes := src.ListMailboxes(got.ID)
	if len(boxes) != 1 || boxes[0].PasswordHash != "hash-info" {
		t.Fatalf("mailbox hash: %+v", boxes)
	}
	var job store.Job
	for _, j := range src.ListJobs("queued", 20) {
		if j.Type == "account.reconcile" && j.ResourceID == got.ID {
			job = j
			break
		}
	}
	if job.ID == "" {
		t.Fatal("expected reconcile job")
	}
	if strAny(job.Payload["copy_source"]) != "/home/acme42" {
		t.Fatalf("copy_source %v", job.Payload)
	}
}

func strAny(v any) string {
	s, _ := v.(string)
	return s
}

func TestImportRejectsCollision(t *testing.T) {
	st := store.NewMemory()
	st.PutAccount(&store.Account{ID: "a", Username: "acme42", PrimaryDomain: "acme.test"})
	exp := HostingAccountExport{FormatVersion: 1, Account: store.Account{ID: "b", Username: "acme42", PrimaryDomain: "other.test"}}
	raw, _ := json.Marshal(exp)
	if _, err := Import(st, raw); err == nil {
		t.Fatal("expected collision")
	}
}
