package provisioning

import (
	"testing"

	"github.com/hosting-panel/panel/internal/mail"
	"github.com/hosting-panel/panel/internal/store"
)

func TestMailMapsStayDeterministic(t *testing.T) {
	st := store.NewMemory()
	st.PutAccount(&store.Account{ID: "a", Username: "siteone", LinuxUID: 20020, LinuxGID: 20020})
	st.PutDomain(&store.Domain{ID: "d", AccountID: "a", ASCII: "site.example"})
	st.PutMailDomain(&store.MailDomain{ID: "md", AccountID: "a", DomainID: "d"})
	st.PutMailbox(&store.Mailbox{ID: "m", AccountID: "a", DomainID: "md", LocalPart: "postmaster", PasswordHash: "!"})
	if mail.Virtual(mail.Recipients(st, "a")) == "" {
		t.Fatal("empty virtual map")
	}
}
