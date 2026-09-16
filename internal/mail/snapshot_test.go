package mail

import (
	"strings"
	"testing"

	"github.com/hosting-panel/panel/internal/store"
)

type countingStore struct {
	store.Store
	accounts int
}

func (c *countingStore) ListAccounts(q, status string) []store.Account {
	c.accounts++
	return c.Store.ListAccounts(q, status)
}

func TestSnapshotHostUsesOneAccountScan(t *testing.T) {
	inner := store.NewMemory()
	inner.PutPackage(&store.Package{ID: "p", EmailDailyLimit: 10})
	inner.PutAccount(&store.Account{ID: "a", Username: "acme", LinuxUID: 20010, LinuxGID: 20010, Status: "active", PackageID: "p"})
	inner.PutAccount(&store.Account{ID: "b", Username: "beta", LinuxUID: 20011, LinuxGID: 20011, Status: "active", PackageID: "p"})
	inner.PutDomain(&store.Domain{ID: "da", AccountID: "a", ASCII: "acme.test"})
	inner.PutDomain(&store.Domain{ID: "db", AccountID: "b", ASCII: "beta.test"})
	inner.PutMailDomain(&store.MailDomain{ID: "mda", AccountID: "a", DomainID: "da"})
	inner.PutMailDomain(&store.MailDomain{ID: "mdb", AccountID: "b", DomainID: "db", CatchallPolicy: "info"})
	inner.PutMailbox(&store.Mailbox{ID: "ma", AccountID: "a", DomainID: "mda", LocalPart: "info", PasswordHash: "ha"})
	inner.PutMailbox(&store.Mailbox{ID: "mb", AccountID: "b", DomainID: "mdb", LocalPart: "sales", PasswordHash: "hb"})
	inner.PutMailAlias(&store.MailAlias{ID: "al", AccountID: "a", DomainID: "mda", Address: "sales", Destination: "info"})
	counter := &countingStore{Store: inner}
	snap := SnapshotHost(counter)
	if counter.accounts != 1 {
		t.Fatalf("account scans=%d", counter.accounts)
	}
	if len(snap.Recipients) != 2 || !strings.Contains(snap.Virtual(), "info@acme.test") {
		t.Fatalf("snapshot recipients: %+v maps=%s", snap.Recipients, snap.Virtual())
	}
	if !strings.Contains(snap.AliasMap, "sales@acme.test") || !strings.Contains(snap.CatchallVirtual, "@beta.test") {
		t.Fatalf("maps: alias=%q catchall=%q", snap.AliasMap, snap.CatchallVirtual)
	}
}
