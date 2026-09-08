package migration

import (
	"encoding/json"
	"time"

	"github.com/hosting-panel/panel/internal/store"
)

type HostingAccountExport struct {
	FormatVersion int                    `json:"format_version"`
	ExportedAt    string                 `json:"exported_at"`
	Account       store.Account          `json:"account"`
	Domains       []store.Domain         `json:"domains"`
	Websites      []store.Website        `json:"websites"`
	Databases     []store.HostedDatabase `json:"databases"`
	Mailboxes     []store.Mailbox        `json:"mailboxes"`
	MailDomains   []store.MailDomain     `json:"mail_domains"`
	Zones         []store.DNSZone        `json:"dns_zones"`
	Records       []store.DNSRecord      `json:"dns_records"`
	Cron          []store.CronJob        `json:"cron"`
	FTP           []store.FTPAccount     `json:"ftp"`
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
		Databases:     st.ListDBs(accountID),
		Mailboxes:     st.ListMailboxes(accountID),
		MailDomains:   st.ListMailDomains(accountID),
		Zones:         st.ListZones(accountID),
		Cron:          st.ListCrons(accountID),
		FTP:           st.ListFTP(accountID),
	}
	for _, z := range exp.Zones {
		exp.Records = append(exp.Records, st.ListRecords(z.ID)...)
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
	if username != "" {
		exp.Account.Username = username
		exp.Account.HomePath = "/home/" + username
		exp.Account.LinuxUID = 0
		exp.Account.LinuxGID = 0
		exp.Account.OwnerUserID = ""
	}
	if domain != "" {
		old := exp.Account.PrimaryDomain
		exp.Account.PrimaryDomain = domain
		for i := range exp.Domains {
			if exp.Domains[i].ASCII == old || exp.Domains[i].Type == "primary" {
				exp.Domains[i].FQDN = domain
				exp.Domains[i].ASCII = domain
				exp.Domains[i].DocumentRoot = "/home/" + exp.Account.Username + "/public_html"
			}
		}
		for i := range exp.Zones {
			if exp.Zones[i].Name == old {
				exp.Zones[i].Name = domain
			}
		}
	}
	if username != "" || domain != "" {
		remapExportIDs(&exp)
	}
	if st.AccountByUsername(exp.Account.Username) != nil {
		return nil, errMissing("username taken")
	}
	if st.DomainTaken(exp.Account.PrimaryDomain) {
		return nil, errMissing("domain collision")
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
	acc.Status = "provisioning"
	acc.DesiredRevision = acc.ObservedRevision + 1
	st.PutAccount(&acc)
	for i := range exp.Domains {
		st.PutDomain(&exp.Domains[i])
	}
	for i := range exp.Websites {
		st.PutWebsite(&exp.Websites[i])
	}
	for i := range exp.Databases {
		st.PutDB(&exp.Databases[i])
	}
	for i := range exp.MailDomains {
		st.PutMailDomain(&exp.MailDomains[i])
	}
	for i := range exp.Mailboxes {
		st.PutMailbox(&exp.Mailboxes[i])
	}
	for i := range exp.Zones {
		st.PutZone(&exp.Zones[i])
	}
	for i := range exp.Records {
		st.PutRecord(&exp.Records[i])
	}
	_, _ = st.EnqueueJob(&store.Job{Type: "account.reconcile", ResourceType: "account", ResourceID: acc.ID, Payload: map[string]any{"account_id": acc.ID}, State: "queued"})
	return &acc, nil
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
}

type missing string

func (m missing) Error() string { return string(m) }
func errMissing(s string) error { return missing(s) }
