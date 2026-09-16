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
	return SnapshotHost(st).DomainNames
}

func VDomains(st store.Store) string {
	var b strings.Builder
	for _, n := range MailDomainNames(st) {
		fmt.Fprintf(&b, "%s OK\n", n)
	}
	return b.String()
}

func CatchallVirtual(st store.Store) string {
	return SnapshotHost(st).CatchallVirtual
}

// RecipientsForHost builds the global virtual/passwd maps. ApplyMailMaps
// replaces the files on disk, so one account must not erase the others.
func RecipientsForHost(st store.Store) []Recipient {
	return SnapshotHost(st).Recipients
}

func AliasMap(st store.Store) string {
	return SnapshotHost(st).AliasMap
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

func SenderLogin(st store.Store, recs []Recipient) string {
	var lines []string
	seen := map[string]bool{}
	add := func(addr, login string) {
		if addr == "" || login == "" {
			return
		}
		key := addr + " " + login
		if seen[key] {
			return
		}
		seen[key] = true
		lines = append(lines, addr+" "+login+"\n")
	}
	for _, r := range recs {
		add(r.Address, r.Address)
	}
	for _, acc := range st.ListAccounts("", "") {
		if acc.Status == "terminated" || acc.Status == "terminating" {
			continue
		}
		for _, al := range st.ListMailAliases(acc.ID) {
			md := mailDomain(st, al.DomainID)
			if md == nil {
				continue
			}
			d := st.GetDomain(md.DomainID)
			if d == nil || d.ASCII == "" || al.Address == "" || al.Destination == "" {
				continue
			}
			for _, part := range strings.Split(al.Destination, ",") {
				dest := strings.TrimSpace(part)
				if dest == "" {
					continue
				}
				if !strings.Contains(dest, "@") {
					dest = dest + "@" + d.ASCII
				}
				add(al.Address+"@"+d.ASCII, dest)
			}
		}
	}
	sort.Strings(lines)
	return "# panel sender-login map — generated, do not edit\n" + strings.Join(lines, "")
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

type HostSnapshot struct {
	Recipients      []Recipient
	DomainNames     []string
	CatchallVirtual string
	AliasMap        string
	SenderLogin     string
}

func SnapshotHost(st store.Store) HostSnapshot {
	accounts := map[string]store.Account{}
	for _, acc := range st.ListAccounts("", "") {
		accounts[acc.ID] = acc
	}
	packages := map[string]store.Package{}
	for _, pkg := range st.ListPackages() {
		packages[pkg.ID] = pkg
	}
	domains := map[string]store.Domain{}
	for _, domain := range st.ListDomains("") {
		domains[domain.ID] = domain
	}
	mailDomains := map[string]store.MailDomain{}
	for _, md := range st.ListMailDomains("") {
		mailDomains[md.ID] = md
	}
	var recs []Recipient
	seenAddr := map[string]bool{}
	for _, mb := range st.ListMailboxes("") {
		acc, ok := accounts[mb.AccountID]
		if !ok || acc.Status == "terminated" || acc.Status == "terminating" {
			continue
		}
		md := mailDomains[mb.DomainID]
		d := domains[md.DomainID]
		if md.ID == "" || d.ID == "" || d.ASCII == "" {
			continue
		}
		hash := mb.PasswordHash
		if hash == "" {
			hash = "!"
		}
		daily := 0
		if pkg, ok := packages[acc.PackageID]; ok {
			daily = pkg.EmailDailyLimit
		}
		rec := Recipient{
			Address: mb.LocalPart + "@" + d.ASCII, Domain: d.ASCII, LocalPart: mb.LocalPart,
			Home: "/var/vmail/" + d.ASCII + "/" + mb.LocalPart, UID: acc.LinuxUID, GID: acc.LinuxGID,
			Quota: mb.QuotaBytes, Hash: hash, Account: acc.Username, DailyLimit: daily,
		}
		if seenAddr[rec.Address] {
			continue
		}
		seenAddr[rec.Address] = true
		recs = append(recs, rec)
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].Address < recs[j].Address })
	seenDomain := map[string]bool{}
	var names []string
	var catchalls []string
	for _, md := range mailDomains {
		acc, ok := accounts[md.AccountID]
		if !ok || acc.Status == "terminated" || acc.Status == "terminating" {
			continue
		}
		d := domains[md.DomainID]
		if d.ASCII == "" {
			continue
		}
		if !seenDomain[d.ASCII] {
			seenDomain[d.ASCII] = true
			names = append(names, d.ASCII)
		}
		if local := CatchallLocal(md.CatchallPolicy); local != "" {
			catchalls = append(catchalls, fmt.Sprintf("@%s %s/%s/Maildir/\n", d.ASCII, d.ASCII, local))
		}
	}
	sort.Strings(names)
	sort.Strings(catchalls)
	var aliasLines []string
	senderLines := []string{}
	senderSeen := map[string]bool{}
	addSender := func(addr, login string) {
		if addr == "" || login == "" {
			return
		}
		key := addr + " " + login
		if senderSeen[key] {
			return
		}
		senderSeen[key] = true
		senderLines = append(senderLines, addr+" "+login+"\n")
	}
	for _, rec := range recs {
		addSender(rec.Address, rec.Address)
	}
	for _, alias := range st.ListMailAliases("") {
		acc, ok := accounts[alias.AccountID]
		if !ok || acc.Status == "terminated" || acc.Status == "terminating" {
			continue
		}
		md := mailDomains[alias.DomainID]
		d := domains[md.DomainID]
		if md.ID == "" || d.ASCII == "" || alias.Address == "" || alias.Destination == "" {
			continue
		}
		var dests []string
		for _, part := range strings.Split(alias.Destination, ",") {
			dest := strings.TrimSpace(part)
			if dest == "" {
				continue
			}
			if !strings.Contains(dest, "@") {
				dest = dest + "@" + d.ASCII
			}
			dests = append(dests, dest)
			addSender(alias.Address+"@"+d.ASCII, dest)
		}
		if len(dests) == 0 {
			continue
		}
		aliasLines = append(aliasLines, fmt.Sprintf("%s@%s %s\n", alias.Address, d.ASCII, strings.Join(dests, ",")))
	}
	sort.Strings(aliasLines)
	sort.Strings(senderLines)
	return HostSnapshot{
		Recipients:      recs,
		DomainNames:     names,
		CatchallVirtual: strings.Join(catchalls, ""),
		AliasMap:        "# panel virtual alias map — generated, do not edit\n" + strings.Join(aliasLines, ""),
		SenderLogin:     "# panel sender-login map — generated, do not edit\n" + strings.Join(senderLines, ""),
	}
}

func (s HostSnapshot) Virtual() string {
	return Virtual(s.Recipients) + s.CatchallVirtual
}

func (s HostSnapshot) VDomains() string {
	var b strings.Builder
	for _, n := range s.DomainNames {
		fmt.Fprintf(&b, "%s OK\n", n)
	}
	return b.String()
}
