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
	src.PutWebsite(&store.Website{ID: "w1", AccountID: "acc1", DomainID: "d1", Runtime: "php", DocumentRoot: "/home/acme42/public_html"})
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
	if len(sites) == 0 || sites[0].DocumentRoot != "/home/moved42/public_html" {
		t.Fatalf("document root not remapped: %+v", sites)
	}
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
