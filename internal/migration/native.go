package migration

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/hosting-panel/panel/internal/store"
)

type HostingAccountExport struct {
	FormatVersion int                    `json:"format_version"`
	ExportedAt    string                 `json:"exported_at"`
	Account       store.Account          `json:"account"`
	Domains       []store.Domain         `json:"domains"`
	Websites      []store.Website        `json:"websites"`
	Applications  []store.Application    `json:"applications,omitempty"`
	Databases     []store.HostedDatabase `json:"databases"`
	Mailboxes     []store.Mailbox        `json:"mailboxes"`
	MailboxHashes map[string]string      `json:"mailbox_hashes,omitempty"`
	MailDomains   []store.MailDomain     `json:"mail_domains"`
	Aliases       []store.MailAlias      `json:"mail_aliases,omitempty"`
	Zones         []store.DNSZone        `json:"dns_zones"`
	Records       []store.DNSRecord      `json:"dns_records"`
	Cron          []store.CronJob        `json:"cron"`
	FTP           []store.FTPAccount     `json:"ftp"`
	FTPHashes     map[string]string      `json:"ftp_hashes,omitempty"`
	SSH           []store.SSHKey         `json:"ssh_keys,omitempty"`
	Homedir       string                 `json:"homedir,omitempty"`
}

func Export(st store.Store, accountID string) (*HostingAccountExport, error) {
	acc := st.GetAccount(accountID)
	if acc == nil {
		return nil, errMissing("account")
	}
	exp := &HostingAccountExport{
		FormatVersion: 1,
		ExportedAt:    time.Now().UTC().Format(time.RFC3339),
		Account:       *acc,
		Domains:       st.ListDomains(accountID),
		Websites:      st.ListWebsites(accountID),
		Applications:  st.ListApps(accountID),
		Databases:     st.ListDBs(accountID),
		Mailboxes:     st.ListMailboxes(accountID),
		MailboxHashes: map[string]string{},
		MailDomains:   st.ListMailDomains(accountID),
		Aliases:       st.ListMailAliases(accountID),
		Zones:         st.ListZones(accountID),
		Cron:          st.ListCrons(accountID),
		FTP:           st.ListFTP(accountID),
		FTPHashes:     map[string]string{},
		SSH:           st.ListSSH(accountID),
	}
	for _, z := range exp.Zones {
		exp.Records = append(exp.Records, st.ListRecords(z.ID)...)
	}
	for _, mb := range exp.Mailboxes {
		if mb.PasswordHash == "" {
			continue
		}
		exp.MailboxHashes[mailboxKey(exp, mb)] = mb.PasswordHash
	}
	for _, f := range exp.FTP {
		if f.PasswordHash == "" {
			continue
		}
		exp.FTPHashes[f.Username] = f.PasswordHash
	}
	return exp, nil
}

func Import(st store.Store, raw []byte) (*store.Account, error) {
	return ImportAs(st, raw, "", "", "")
}

func ImportAs(st store.Store, raw []byte, username, domain, ownerUserID string) (*store.Account, error) {
	var exp HostingAccountExport
	if err := json.Unmarshal(raw, &exp); err != nil {
		return nil, err
	}
	if exp.FormatVersion != 1 {
		return nil, errMissing("format")
	}
	applySecretHashes(&exp)
	srcUser := exp.Account.Username
	srcDomain := exp.Account.PrimaryDomain
	if username != "" {
		exp.Account.Username = username
		exp.Account.HomePath = "/home/" + username
		exp.Account.LinuxUID = 0
		exp.Account.LinuxGID = 0
		exp.Account.OwnerUserID = ""
	}
	if domain != "" {
		exp.Account.PrimaryDomain = domain
	}
	needRemap := username != "" || domain != ""
	if needRemap {
		rewriteExportTree(&exp, srcUser, exp.Account.Username, srcDomain, exp.Account.PrimaryDomain)
	}
	plan := planDataMove(&exp, srcUser, srcDomain, exp.Account.Username, exp.Account.PrimaryDomain)
	if needRemap {
		remapExportIDs(&exp)
	}
	if st.AccountByUsername(exp.Account.Username) != nil {
		return nil, errMissing("username taken")
	}
	if st.DomainTaken(exp.Account.PrimaryDomain) {
		return nil, errMissing("domain collision")
	}
	for _, d := range exp.Domains {
		if d.ASCII != "" && st.DomainTaken(d.ASCII) {
			return nil, errMissing("domain collision")
		}
	}
	acc := exp.Account
	if acc.LinuxUID < 20000 {
		acc.LinuxUID = st.AllocUID()
		acc.LinuxGID = acc.LinuxUID
	}
	if acc.HomePath == "" {
		acc.HomePath = "/home/" + acc.Username
	}
	if ownerUserID != "" {
		acc.OwnerUserID = ownerUserID
	}
	if acc.OwnerUserID == "" {
		return nil, errMissing("owner")
	}
	if acc.PackageID == "" {
		pkgs := st.ListPackages()
		if len(pkgs) == 0 {
			return nil, errMissing("package")
		}
		acc.PackageID = pkgs[0].ID
	}
	acc.Status = "provisioning"
	acc.DesiredRevision = acc.ObservedRevision + 1
	st.PutAccount(&acc)
	if st.GetAccount(acc.ID) == nil {
		return nil, errMissing("account persist")
	}
	for i := range exp.Domains {
		st.PutDomain(&exp.Domains[i])
	}
	for i := range exp.Websites {
		st.PutWebsite(&exp.Websites[i])
	}
	for i := range exp.Applications {
		st.PutApp(&exp.Applications[i])
	}
	engines := map[string]bool{}
	for i := range exp.Databases {
		st.PutDB(&exp.Databases[i])
		if exp.Databases[i].Engine != "" {
			engines[exp.Databases[i].Engine] = true
		}
	}
	if len(engines) > 0 && len(st.ListDBUsers(acc.ID)) == 0 {
		for engine := range engines {
			st.PutDBUser(&store.DatabaseUser{
				ID: store.NewID(), AccountID: acc.ID,
				Username: acc.Username + "_u", Engine: engine,
			})
		}
	}
	for i := range exp.MailDomains {
		st.PutMailDomain(&exp.MailDomains[i])
	}
	for i := range exp.Mailboxes {
		st.PutMailbox(&exp.Mailboxes[i])
	}
	for i := range exp.Aliases {
		st.PutMailAlias(&exp.Aliases[i])
	}
	for i := range exp.Zones {
		st.PutZone(&exp.Zones[i])
	}
	for i := range exp.Records {
		st.PutRecord(&exp.Records[i])
	}
	for i := range exp.Cron {
		st.PutCron(&exp.Cron[i])
	}
	for i := range exp.SSH {
		st.PutSSH(&exp.SSH[i])
	}
	for i := range exp.FTP {
		ftp := exp.FTP[i]
		if st.FTPUsernameTaken(ftp.Username, ftp.ID) {
			ftp.Username = acc.Username + "_" + ftp.Username
		}
		st.PutFTP(&ftp)
	}
	plan["account_id"] = acc.ID
	plan["username"] = acc.Username
	plan["copy_dest"] = acc.HomePath
	_, _ = st.EnqueueJob(&store.Job{
		Type: "account.reconcile", ResourceType: "account", ResourceID: acc.ID,
		Payload: plan, State: "queued",
	})
	return &acc, nil
}

func applySecretHashes(exp *HostingAccountExport) {
	for i := range exp.Mailboxes {
		if exp.Mailboxes[i].PasswordHash != "" {
			continue
		}
		if h := exp.MailboxHashes[mailboxKey(exp, exp.Mailboxes[i])]; h != "" {
			exp.Mailboxes[i].PasswordHash = h
		}
	}
	for i := range exp.FTP {
		if exp.FTP[i].PasswordHash != "" {
			continue
		}
		if h := exp.FTPHashes[exp.FTP[i].Username]; h != "" {
			exp.FTP[i].PasswordHash = h
		}
	}
}

func mailboxKey(exp *HostingAccountExport, mb store.Mailbox) string {
	ascii := ""
	for _, md := range exp.MailDomains {
		if md.ID == mb.DomainID {
			for _, d := range exp.Domains {
				if d.ID == md.DomainID {
					ascii = d.ASCII
					break
				}
			}
			break
		}
	}
	if ascii == "" {
		for _, d := range exp.Domains {
			if d.ID == mb.DomainID {
				ascii = d.ASCII
				break
			}
		}
	}
	if ascii == "" {
		ascii = exp.Account.PrimaryDomain
	}
	return mb.LocalPart + "@" + ascii
}

func planDataMove(exp *HostingAccountExport, srcUser, srcDomain, destUser, destDomain string) map[string]any {
	srcHome := exp.Homedir
	if srcHome == "" {
		srcHome = "/home/" + srcUser
	}
	domainByID := map[string]string{}
	for _, d := range exp.Domains {
		// After rewrite, ASCII is already dest; recover source via inverse map.
		srcASCII := rewriteFQDN(d.ASCII, destDomain, srcDomain)
		if destDomain == srcDomain {
			srcASCII = d.ASCII
		}
		domainByID[d.ID] = srcASCII
	}
	mdASCII := map[string]string{}
	for _, md := range exp.MailDomains {
		mdASCII[md.ID] = domainByID[md.DomainID]
	}
	var dbs []any
	for _, d := range exp.Databases {
		srcName := rewritePrefixedName(d.Name, destUser, srcUser)
		if destUser == srcUser {
			srcName = d.Name
		}
		dbs = append(dbs, map[string]any{"engine": d.Engine, "source": srcName, "dest": d.Name})
	}
	var mails []any
	for _, mb := range exp.Mailboxes {
		srcASCII := mdASCII[mb.DomainID]
		if srcASCII == "" {
			srcASCII = domainByID[mb.DomainID]
		}
		if srcASCII == "" {
			srcASCII = srcDomain
		}
		destASCII := rewriteFQDN(srcASCII, srcDomain, destDomain)
		mails = append(mails, map[string]any{
			"source": "/var/vmail/" + srcASCII + "/" + mb.LocalPart,
			"dest":   "/var/vmail/" + destASCII + "/" + mb.LocalPart,
		})
	}
	return map[string]any{
		"copy_source": srcHome,
		"copy_dest":   "/home/" + destUser,
		"username":    destUser,
		"databases":   dbs,
		"mailboxes":   mails,
	}
}

func rewriteExportTree(exp *HostingAccountExport, oldUser, newUser, oldDom, newDom string) {
	for i := range exp.Domains {
		exp.Domains[i].FQDN = rewriteFQDN(exp.Domains[i].FQDN, oldDom, newDom)
		exp.Domains[i].ASCII = rewriteFQDN(exp.Domains[i].ASCII, oldDom, newDom)
		exp.Domains[i].DocumentRoot = rewriteHomePath(exp.Domains[i].DocumentRoot, oldUser, newUser, oldDom, newDom)
	}
	for i := range exp.Websites {
		exp.Websites[i].DocumentRoot = rewriteHomePath(exp.Websites[i].DocumentRoot, oldUser, newUser, oldDom, newDom)
	}
	for i := range exp.Applications {
		exp.Applications[i].WorkingDirectory = rewriteHomePath(exp.Applications[i].WorkingDirectory, oldUser, newUser, oldDom, newDom)
		exp.Applications[i].ListenTarget = rewriteHomePath(exp.Applications[i].ListenTarget, oldUser, newUser, oldDom, newDom)
	}
	for i := range exp.Zones {
		exp.Zones[i].Name = rewriteFQDN(exp.Zones[i].Name, oldDom, newDom)
	}
	for i := range exp.Records {
		exp.Records[i].Name = rewriteHostToken(exp.Records[i].Name, oldDom, newDom)
		exp.Records[i].Content = rewriteHostToken(exp.Records[i].Content, oldDom, newDom)
	}
	for i := range exp.Databases {
		exp.Databases[i].Name = rewritePrefixedName(exp.Databases[i].Name, oldUser, newUser)
	}
	for i := range exp.Aliases {
		exp.Aliases[i].Destination = rewriteAddress(exp.Aliases[i].Destination, oldDom, newDom)
	}
	for i := range exp.Cron {
		exp.Cron[i].WorkingDirectory = rewriteHomePath(exp.Cron[i].WorkingDirectory, oldUser, newUser, oldDom, newDom)
	}
	for i := range exp.FTP {
		exp.FTP[i].HomePath = rewriteHomePath(exp.FTP[i].HomePath, oldUser, newUser, oldDom, newDom)
		if strings.HasPrefix(exp.FTP[i].Username, oldUser+"_") {
			exp.FTP[i].Username = newUser + "_" + strings.TrimPrefix(exp.FTP[i].Username, oldUser+"_")
		}
	}
}

func rewriteFQDN(name, oldRoot, newRoot string) string {
	if name == "" || oldRoot == "" || oldRoot == newRoot {
		return name
	}
	if name == oldRoot {
		return newRoot
	}
	if strings.HasSuffix(name, "."+oldRoot) {
		return strings.TrimSuffix(name, oldRoot) + newRoot
	}
	return name
}

func rewriteHomePath(p, oldUser, newUser, oldDom, newDom string) string {
	if p == "" {
		return p
	}
	if oldUser != "" && oldUser != newUser {
		old := "/home/" + oldUser
		neu := "/home/" + newUser
		if p == old || strings.HasPrefix(p, old+"/") {
			p = neu + strings.TrimPrefix(p, old)
		}
	}
	if oldDom != "" && oldDom != newDom {
		p = strings.ReplaceAll(p, oldDom, newDom)
	}
	return p
}

func rewritePrefixedName(name, oldUser, newUser string) string {
	if name == "" || oldUser == "" || oldUser == newUser {
		return name
	}
	prefix := oldUser + "_"
	if strings.HasPrefix(name, prefix) {
		return newUser + "_" + strings.TrimPrefix(name, prefix)
	}
	return name
}

func rewriteHostToken(s, oldRoot, newRoot string) string {
	if s == "" || oldRoot == "" || oldRoot == newRoot {
		return s
	}
	if s == oldRoot || strings.HasSuffix(s, "."+oldRoot) {
		return rewriteFQDN(s, oldRoot, newRoot)
	}
	if strings.Contains(s, oldRoot) {
		return strings.ReplaceAll(s, oldRoot, newRoot)
	}
	return s
}

func rewriteAddress(addr, oldRoot, newRoot string) string {
	local, host, ok := strings.Cut(addr, "@")
	if !ok {
		return rewriteFQDN(addr, oldRoot, newRoot)
	}
	return local + "@" + rewriteFQDN(host, oldRoot, newRoot)
}

func remapExportIDs(exp *HostingAccountExport) {
	ids := map[string]string{}
	next := func(old string) string {
		if old == "" {
			return store.NewID()
		}
		if n, ok := ids[old]; ok {
			return n
		}
		n := store.NewID()
		ids[old] = n
		return n
	}
	exp.Account.ID = next(exp.Account.ID)
	for i := range exp.Domains {
		exp.Domains[i].ID = next(exp.Domains[i].ID)
		exp.Domains[i].AccountID = exp.Account.ID
	}
	for i := range exp.Websites {
		exp.Websites[i].ID = next(exp.Websites[i].ID)
		exp.Websites[i].AccountID = exp.Account.ID
		exp.Websites[i].DomainID = next(exp.Websites[i].DomainID)
	}
	for i := range exp.Applications {
		exp.Applications[i].ID = next(exp.Applications[i].ID)
		exp.Applications[i].AccountID = exp.Account.ID
		exp.Applications[i].WebsiteID = next(exp.Applications[i].WebsiteID)
	}
	for i := range exp.Databases {
		exp.Databases[i].ID = next(exp.Databases[i].ID)
		exp.Databases[i].AccountID = exp.Account.ID
	}
	for i := range exp.MailDomains {
		exp.MailDomains[i].ID = next(exp.MailDomains[i].ID)
		exp.MailDomains[i].AccountID = exp.Account.ID
		exp.MailDomains[i].DomainID = next(exp.MailDomains[i].DomainID)
	}
	for i := range exp.Mailboxes {
		exp.Mailboxes[i].ID = next(exp.Mailboxes[i].ID)
		exp.Mailboxes[i].AccountID = exp.Account.ID
		exp.Mailboxes[i].DomainID = next(exp.Mailboxes[i].DomainID)
	}
	for i := range exp.Aliases {
		exp.Aliases[i].ID = next(exp.Aliases[i].ID)
		exp.Aliases[i].AccountID = exp.Account.ID
		exp.Aliases[i].DomainID = next(exp.Aliases[i].DomainID)
	}
	for i := range exp.Zones {
		exp.Zones[i].ID = next(exp.Zones[i].ID)
		exp.Zones[i].AccountID = exp.Account.ID
		exp.Zones[i].DomainID = next(exp.Zones[i].DomainID)
	}
	for i := range exp.Records {
		exp.Records[i].ID = next(exp.Records[i].ID)
		exp.Records[i].ZoneID = next(exp.Records[i].ZoneID)
	}
	for i := range exp.Cron {
		exp.Cron[i].ID = next(exp.Cron[i].ID)
		exp.Cron[i].AccountID = exp.Account.ID
	}
	for i := range exp.FTP {
		exp.FTP[i].ID = next(exp.FTP[i].ID)
		exp.FTP[i].AccountID = exp.Account.ID
	}
	for i := range exp.SSH {
		exp.SSH[i].ID = next(exp.SSH[i].ID)
		exp.SSH[i].AccountID = exp.Account.ID
	}
}

type missing string

func (m missing) Error() string { return string(m) }
func errMissing(s string) error { return missing(s) }
