package store

import (
	"testing"
	"time"

	"github.com/hosting-panel/panel/internal/id"
)

func TestPurgeAccountFreesUsernameAndDomain(t *testing.T) {
	st := NewMemory()
	if err := SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	owner := &User{
		ID: id.New(), Username: "acme42", Email: "ops@acme.test",
		DisplayName: "acme42", Status: "active", Roles: []string{"customer_owner"},
		CreatedAt: time.Now().UTC(),
	}
	st.PutUser(owner)
	acc := &Account{
		ID: id.New(), OwnerUserID: owner.ID, Username: "acme42",
		PrimaryDomain: "acme.test", PackageID: st.ListPackages()[0].ID,
		Status: "terminated", HomePath: "/home/acme42", LinuxUID: 20010, LinuxGID: 20010,
	}
	st.PutAccount(acc)
	st.PutDomain(&Domain{
		ID: id.New(), AccountID: acc.ID, FQDN: "acme.test", ASCII: "acme.test",
		Type: "primary", Status: "terminated",
	})
	st.PutDB(&HostedDatabase{ID: id.New(), AccountID: acc.ID, Name: "acme42_db", Engine: "mariadb", Status: "terminated"})

	if err := st.PurgeAccount(acc.ID); err != nil {
		t.Fatal(err)
	}
	if st.GetAccount(acc.ID) != nil {
		t.Fatal("account row remains")
	}
	if st.AccountByUsername("acme42") != nil {
		t.Fatal("username still reserved")
	}
	if st.UserByUsername("acme42") != nil {
		t.Fatal("owner login still reserved")
	}
	if st.DomainTaken("acme.test") {
		t.Fatal("domain still reserved")
	}
	if st.UserByUsername("admin") == nil {
		t.Fatal("purged the administrator")
	}
}

func TestPurgeAccountMissingIsNoop(t *testing.T) {
	st := NewMemory()
	if err := st.PurgeAccount("missing"); err != nil {
		t.Fatal(err)
	}
}
