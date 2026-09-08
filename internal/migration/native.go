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
	var exp HostingAccountExport
	if err := json.Unmarshal(raw, &exp); err != nil {
		return nil, err
	}
	if exp.FormatVersion != 1 {
		return nil, errMissing("format")
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

type missing string

func (m missing) Error() string { return string(m) }
func errMissing(s string) error { return missing(s) }
