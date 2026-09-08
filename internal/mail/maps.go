package mail

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hosting-panel/panel/internal/store"
)

type Recipient struct {
	Address    string
	Domain     string
	LocalPart  string
	Home       string
	UID        int
	GID        int
	Quota      int64
	Hash       string
	Account    string
	DailyLimit int
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
		account := ""
		daily := 0
		if acc != nil {
			uid, gid = acc.LinuxUID, acc.LinuxGID
			account = acc.Username
			if pkg := st.GetPackage(acc.PackageID); pkg != nil {
				daily = pkg.EmailDailyLimit
			}
		}
		hash := mb.PasswordHash
		if hash == "" {
			hash = "!"
		}
		out = append(out, Recipient{
			Address:    mb.LocalPart + "@" + d.ASCII,
			Domain:     d.ASCII,
			LocalPart:  mb.LocalPart,
			Home:       "/var/vmail/" + d.ASCII + "/" + mb.LocalPart,
			UID:        uid,
			GID:        gid,
			Quota:      mb.QuotaBytes,
			Hash:       hash,
			Account:    account,
			DailyLimit: daily,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Address < out[j].Address })
	return out
}

func CatchallLocal(policy string) string {
	p := strings.TrimSpace(strings.ToLower(policy))
	switch p {
	case "", "reject":
		return ""
	case "discard":
		return "discard"
	default:
		return p
	}
}

func MailDomainNames(st store.Store) []string {
	seen := map[string]bool{}
	var names []string
	for _, acc := range st.ListAccounts("", "") {
		if acc.Status == "terminated" || acc.Status == "terminating" {
			continue
		}
		for _, md := range st.ListMailDomains(acc.ID) {
			d := st.GetDomain(md.DomainID)
			if d == nil || d.ASCII == "" || seen[d.ASCII] {
				continue
			}
			seen[d.ASCII] = true
			names = append(names, d.ASCII)
		}
	}
	sort.Strings(names)
	return names
}

func VDomains(st store.Store) string {
	var b strings.Builder
	for _, n := range MailDomainNames(st) {
		fmt.Fprintf(&b, "%s OK\n", n)
	}
	return b.String()
}

func CatchallVirtual(st store.Store) string {
	var lines []string
	for _, acc := range st.ListAccounts("", "") {
		if acc.Status == "terminated" || acc.Status == "terminating" {
			continue
		}
		for _, md := range st.ListMailDomains(acc.ID) {
			local := CatchallLocal(md.CatchallPolicy)
			if local == "" {
				continue
			}
			d := st.GetDomain(md.DomainID)
			if d == nil || d.ASCII == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("@%s %s/%s/Maildir/\n", d.ASCII, d.ASCII, local))
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "")
}

// RecipientsForHost builds the global virtual/passwd maps. ApplyMailMaps
// replaces the files on disk, so one account must not erase the others.
func RecipientsForHost(st store.Store) []Recipient {
	var out []Recipient
	seen := map[string]bool{}
	for _, acc := range st.ListAccounts("", "") {
		if acc.Status == "terminated" || acc.Status == "terminating" {
			continue
		}
		for _, r := range Recipients(st, acc.ID) {
			if seen[r.Address] {
				continue
			}
			seen[r.Address] = true
			out = append(out, r)
		}
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
		hash := r.Hash
		if hash == "" || hash == "!" {
			hash = "!"
		} else if !strings.HasPrefix(hash, "{") {
			hash = "{ARGON2ID}" + hash
		}
		fmt.Fprintf(&b, "%s:%s:%d:%d::%s::userdb_quota_rule=*:storage=%dB\n",
			r.Address, hash, r.UID, r.GID, r.Home, r.Quota)
	}
	return b.String()
}

func SendLimits(recs []Recipient) string {
	var b strings.Builder
	b.WriteString("# sender account daily_limit — generated, do not edit\n")
	for _, r := range recs {
		if r.Account == "" {
			continue
		}
		fmt.Fprintf(&b, "%s %s %d\n", r.Address, r.Account, r.DailyLimit)
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
