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
	got, err := Import(dst, raw)
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

func TestImportRejectsCollision(t *testing.T) {
	st := store.NewMemory()
	st.PutAccount(&store.Account{ID: "a", Username: "acme42", PrimaryDomain: "acme.test"})
	exp := HostingAccountExport{FormatVersion: 1, Account: store.Account{ID: "b", Username: "acme42", PrimaryDomain: "other.test"}}
	raw, _ := json.Marshal(exp)
	if _, err := Import(st, raw); err == nil {
		t.Fatal("expected collision")
	}
}
