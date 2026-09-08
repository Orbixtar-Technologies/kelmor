package mail

import (
	"strings"
	"testing"

	"github.com/hosting-panel/panel/internal/store"
)

func TestVirtualMaps(t *testing.T) {
	st := store.NewMemory()
	st.PutPackage(&store.Package{ID: "p", EmailDailyLimit: 40})
	st.PutAccount(&store.Account{ID: "a", Username: "acme42", LinuxUID: 20010, LinuxGID: 20010, PackageID: "p"})
	st.PutDomain(&store.Domain{ID: "d", AccountID: "a", ASCII: "acme.test"})
	st.PutMailDomain(&store.MailDomain{ID: "md", AccountID: "a", DomainID: "d"})
	st.PutMailbox(&store.Mailbox{ID: "m", AccountID: "a", DomainID: "md", LocalPart: "info", PasswordHash: "$argon2id$v=19$m=1,t=1,p=1$aa$bb", QuotaBytes: 100})
	recs := Recipients(st, "a")
	if len(recs) != 1 || recs[0].Address != "info@acme.test" {
		t.Fatalf("%v", recs)
	}
	v := Virtual(recs)
	if !strings.Contains(v, "info@acme.test") || !strings.Contains(v, "acme.test/info/Maildir/") {
		t.Fatal(v)
	}
	if !strings.Contains(UIDMap(recs), "20010") {
		t.Fatal(UIDMap(recs))
	}
	p := PasswdFile(recs)
	if !strings.Contains(p, "$argon2id$") && !strings.Contains(p, "{ARGON2ID}") {
		t.Fatal(p)
	}
	sl := SendLimits(recs)
	if !strings.Contains(sl, "info@acme.test acme42 40") {
		t.Fatal(sl)
	}
}

func TestRecipientsForHostKeepsEveryAccount(t *testing.T) {
	st := store.NewMemory()
	st.PutAccount(&store.Account{ID: "a", Username: "acme", LinuxUID: 20010, LinuxGID: 20010, Status: "active"})
	st.PutAccount(&store.Account{ID: "b", Username: "beta", LinuxUID: 20011, LinuxGID: 20011, Status: "active"})
	st.PutDomain(&store.Domain{ID: "da", AccountID: "a", ASCII: "acme.test"})
	st.PutDomain(&store.Domain{ID: "db", AccountID: "b", ASCII: "beta.test"})
	st.PutMailDomain(&store.MailDomain{ID: "mda", AccountID: "a", DomainID: "da"})
	st.PutMailDomain(&store.MailDomain{ID: "mdb", AccountID: "b", DomainID: "db"})
	st.PutMailbox(&store.Mailbox{ID: "ma", AccountID: "a", DomainID: "mda", LocalPart: "info", PasswordHash: "ha"})
	st.PutMailbox(&store.Mailbox{ID: "mb", AccountID: "b", DomainID: "mdb", LocalPart: "sales", PasswordHash: "hb"})
	recs := RecipientsForHost(st)
	body := PasswdFile(recs)
	if !strings.Contains(body, "info@acme.test") || !strings.Contains(body, "sales@beta.test") {
		t.Fatal(body)
	}
}

func TestCatchallVirtual(t *testing.T) {
	st := store.NewMemory()
	st.PutAccount(&store.Account{ID: "a", Username: "acme", LinuxUID: 20010, LinuxGID: 20010, Status: "active"})
	st.PutDomain(&store.Domain{ID: "d", AccountID: "a", ASCII: "acme.test"})
	st.PutMailDomain(&store.MailDomain{ID: "md", AccountID: "a", DomainID: "d", CatchallPolicy: "info"})
	st.PutMailbox(&store.Mailbox{ID: "m", AccountID: "a", DomainID: "md", LocalPart: "info", PasswordHash: "h"})
	v := Virtual(RecipientsForHost(st)) + CatchallVirtual(st)
	if !strings.Contains(v, "@acme.test acme.test/info/Maildir/") {
		t.Fatal(v)
	}
	if !strings.Contains(VDomains(st), "acme.test OK") {
		t.Fatal(VDomains(st))
	}
	if CatchallLocal("reject") != "" || CatchallLocal("discard") != "discard" {
		t.Fatal(CatchallLocal("discard"))
	}
}

func TestAliasMap(t *testing.T) {
	st := store.NewMemory()
	st.PutAccount(&store.Account{ID: "a", Username: "acme", LinuxUID: 20010, LinuxGID: 20010, Status: "active"})
	st.PutDomain(&store.Domain{ID: "d", AccountID: "a", ASCII: "acme.test"})
	st.PutMailDomain(&store.MailDomain{ID: "md", AccountID: "a", DomainID: "d"})
	st.PutMailbox(&store.Mailbox{ID: "m", AccountID: "a", DomainID: "md", LocalPart: "info", PasswordHash: "h"})
	st.PutMailAlias(&store.MailAlias{ID: "al", AccountID: "a", DomainID: "md", Address: "sales", Destination: "info"})
	body := AliasMap(st)
	if !strings.Contains(body, "sales@acme.test info@acme.test") {
		t.Fatal(body)
	}
	recs := RecipientsForHost(st)
	login := SenderLogin(st, recs)
	if !strings.Contains(login, "info@acme.test info@acme.test") {
		t.Fatal(login)
	}
	if !strings.Contains(login, "sales@acme.test info@acme.test") {
		t.Fatal(login)
	}
}

func TestCatchallSkipsTerminating(t *testing.T) {
	st := store.NewMemory()
	st.PutAccount(&store.Account{ID: "a", Username: "gone", Status: "terminated"})
	st.PutDomain(&store.Domain{ID: "d", AccountID: "a", ASCII: "gone.test"})
	st.PutMailDomain(&store.MailDomain{ID: "md", AccountID: "a", DomainID: "d", CatchallPolicy: "info"})
	if CatchallVirtual(st) != "" {
		t.Fatal(CatchallVirtual(st))
	}
}

func TestRecipientsForHostSkipsTerminating(t *testing.T) {
	st := store.NewMemory()
	st.PutAccount(&store.Account{ID: "a", Username: "acme", LinuxUID: 20010, LinuxGID: 20010, Status: "terminating"})
	st.PutAccount(&store.Account{ID: "b", Username: "beta", LinuxUID: 20011, LinuxGID: 20011, Status: "terminated"})
	st.PutAccount(&store.Account{ID: "c", Username: "keep", LinuxUID: 20012, LinuxGID: 20012, Status: "active"})
	st.PutDomain(&store.Domain{ID: "da", AccountID: "a", ASCII: "acme.test"})
	st.PutDomain(&store.Domain{ID: "db", AccountID: "b", ASCII: "beta.test"})
	st.PutDomain(&store.Domain{ID: "dc", AccountID: "c", ASCII: "keep.test"})
	st.PutMailDomain(&store.MailDomain{ID: "mda", AccountID: "a", DomainID: "da"})
	st.PutMailDomain(&store.MailDomain{ID: "mdb", AccountID: "b", DomainID: "db"})
	st.PutMailDomain(&store.MailDomain{ID: "mdc", AccountID: "c", DomainID: "dc"})
	st.PutMailbox(&store.Mailbox{ID: "ma", AccountID: "a", DomainID: "mda", LocalPart: "info", PasswordHash: "ha"})
	st.PutMailbox(&store.Mailbox{ID: "mb", AccountID: "b", DomainID: "mdb", LocalPart: "sales", PasswordHash: "hb"})
	st.PutMailbox(&store.Mailbox{ID: "mc", AccountID: "c", DomainID: "mdc", LocalPart: "ops", PasswordHash: "hc"})
	body := PasswdFile(RecipientsForHost(st))
	if strings.Contains(body, "info@acme.test") || strings.Contains(body, "sales@beta.test") {
		t.Fatal(body)
	}
	if !strings.Contains(body, "ops@keep.test") {
		t.Fatal(body)
	}
}
