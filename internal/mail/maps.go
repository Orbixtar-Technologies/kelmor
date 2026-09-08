package mail

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hosting-panel/panel/internal/store"
)

type Recipient struct {
	Address   string
	Domain    string
	LocalPart string
	Home      string
	UID       int
	GID       int
	Quota     int64
	Hash      string
}

func Recipients(st store.Store, accountID string) []Recipient {
	var out []Recipient
	for _, mb := range st.ListMailboxes(accountID) {
		md := mailDomain(st, mb.DomainID)
		if md == nil {
			continue
		}
		d := st.GetDomain(md.DomainID)
		if d == nil {
			continue
		}
		acc := st.GetAccount(mb.AccountID)
		uid, gid := 20000, 20000
		if acc != nil {
			uid, gid = acc.LinuxUID, acc.LinuxGID
		}
		hash := mb.PasswordHash
		if hash == "" {
			hash = "!"
		}
		out = append(out, Recipient{
			Address:   mb.LocalPart + "@" + d.ASCII,
			Domain:    d.ASCII,
			LocalPart: mb.LocalPart,
			Home:      "/var/vmail/" + d.ASCII + "/" + mb.LocalPart,
			UID:       uid,
			GID:       gid,
			Quota:     mb.QuotaBytes,
			Hash:      hash,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Address < out[j].Address })
	return out
}

func Virtual(recs []Recipient) string {
	var b strings.Builder
	b.WriteString("# panel virtual mailbox map — generated, do not edit\n")
	for _, r := range recs {
		fmt.Fprintf(&b, "%s %s/%s/Maildir/\n", r.Address, r.Domain, r.LocalPart)
	}
	return b.String()
}

func Domains(recs []Recipient) string {
	seen := map[string]bool{}
	var names []string
	for _, r := range recs {
		if !seen[r.Domain] {
			seen[r.Domain] = true
			names = append(names, r.Domain)
		}
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		fmt.Fprintf(&b, "%s OK\n", n)
	}
	return b.String()
}

func UIDMap(recs []Recipient) string {
	var b strings.Builder
	for _, r := range recs {
		fmt.Fprintf(&b, "%s %d\n", r.Address, r.UID)
	}
	return b.String()
}

func GIDMap(recs []Recipient) string {
	var b strings.Builder
	for _, r := range recs {
		fmt.Fprintf(&b, "%s %d\n", r.Address, r.GID)
	}
	return b.String()
}

func PasswdFile(recs []Recipient) string {
	var b strings.Builder
	b.WriteString("# dovecot passwd-file — generated, do not edit\n")
	for _, r := range recs {
		fmt.Fprintf(&b, "%s:{ARGON2ID}%s:%d:%d::%s::userdb_quota_rule=*:storage=%dB\n",
			r.Address, strings.TrimPrefix(r.Hash, "$argon2id$"), r.UID, r.GID, r.Home, r.Quota)
	}
	return b.String()
}

func mailDomain(st store.Store, id string) *store.MailDomain {
	for _, md := range st.ListMailDomains("") {
		if md.ID == id {
			cp := md
			return &cp
		}
	}
	return nil
}
