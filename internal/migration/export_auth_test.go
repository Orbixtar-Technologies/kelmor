package migration

import (
	"testing"

	"github.com/hosting-panel/panel/internal/store"
)

func TestExportOmitsHashesByDefault(t *testing.T) {
	st := store.NewMemory()
	st.PutAccount(&store.Account{ID: "acc1", Username: "acme42", PrimaryDomain: "acme.test", HomePath: "/home/acme42", Status: "active"})
	st.PutMailbox(&store.Mailbox{ID: "mb1", AccountID: "acc1", DomainID: "md1", LocalPart: "info", PasswordHash: "hash-info", Status: "active"})
	st.PutFTP(&store.FTPAccount{ID: "ftp1", AccountID: "acc1", Username: "acme42_ftp", HomePath: "/home/acme42/public_html", PasswordHash: "hash-ftp", Status: "active"})
	exp, err := Export(st, "acc1")
	if err != nil {
		t.Fatal(err)
	}
	if len(exp.MailboxHashes) != 0 || len(exp.FTPHashes) != 0 {
		t.Fatalf("default export leaked hashes: %+v", exp)
	}
	if len(exp.Mailboxes) != 1 || exp.Mailboxes[0].PasswordHash != "" {
		t.Fatalf("mailbox hash present: %+v", exp.Mailboxes)
	}
	if len(exp.FTP) != 1 || exp.FTP[0].PasswordHash != "" {
		t.Fatalf("ftp hash present: %+v", exp.FTP)
	}
}
