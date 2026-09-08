package mail

import (
	"strings"
	"testing"

	"github.com/hosting-panel/panel/internal/store"
)

func TestVirtualMaps(t *testing.T) {
	st := store.NewMemory()
	st.PutAccount(&store.Account{ID: "a", Username: "acme42", LinuxUID: 20010, LinuxGID: 20010})
	st.PutDomain(&store.Domain{ID: "d", AccountID: "a", ASCII: "acme.test"})
	st.PutMailDomain(&store.MailDomain{ID: "md", AccountID: "a", DomainID: "d"})
	st.PutMailbox(&store.Mailbox{ID: "m", AccountID: "a", DomainID: "md", LocalPart: "info", PasswordHash: "$argon2id$v=19$m=1,t=1,p=1$aa$bb", QuotaBytes: 100})
	recs := Recipients(st, "a")
	if len(recs) != 1 || recs[0].Address != "info@acme.test" {
		t.Fatalf("%v", recs)
	}
	v := Virtual(recs)
	if !strings.Contains(v, "info@acme.test") {
		t.Fatal(v)
	}
	p := PasswdFile(recs)
	if !strings.Contains(p, "{ARGON2ID}") {
		t.Fatal(p)
	}
}
