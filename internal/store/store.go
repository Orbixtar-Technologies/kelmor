package store

type Store interface {
	PutUser(*User)
	UserByUsername(string) *User
	UserByID(string) *User

	PutSession(*Session)
	SessionByHash([]byte) *Session
	RevokeSession(string)

	PutFeature(*FeatureSet)
	ListFeatureSets() []FeatureSet

	PutPackage(*Package)
	GetPackage(string) *Package
	ListPackages() []Package

	PutReseller(*Reseller)
	GetReseller(string) *Reseller
	ResellerByUser(userID string) *Reseller
	ListResellers() []Reseller

	AllocUID() int
	PutAccount(*Account)
	GetAccount(string) *Account
	AccountByUsername(string) *Account
	ListAccounts(q, status string) []Account
	AddMember(accountID, userID string)
	AccountsForUser(userID string) []string
	DriftedAccounts() []Account

	DomainTaken(ascii string) bool
	PutDomain(*Domain)
	GetDomain(string) *Domain
	ListDomains(accountID string) []Domain

	PutWebsite(*Website)
	GetWebsite(string) *Website
	ListWebsites(accountID string) []Website

	PutApp(*Application)
	GetApp(string) *Application
	ListApps(accountID string) []Application

	PutDB(*HostedDatabase)
	GetDB(string) *HostedDatabase
	ListDBs(accountID string) []HostedDatabase
	PutDBUser(*DatabaseUser)
	ListDBUsers(accountID string) []DatabaseUser

	PutZone(*DNSZone)
	GetZone(string) *DNSZone
	ZoneByDomain(domainID string) *DNSZone
	ListZones(accountID string) []DNSZone
	PutRecord(*DNSRecord)
	ListRecords(zoneID string) []DNSRecord
	DeleteRecord(id string)

	PutMailDomain(*MailDomain)
	MailDomainByDomain(domainID string) *MailDomain
	ListMailDomains(accountID string) []MailDomain
	PutMailbox(*Mailbox)
	GetMailbox(string) *Mailbox
	ListMailboxes(accountID string) []Mailbox

	PutCert(*Certificate)
	GetCert(string) *Certificate
	ListCerts(accountID string) []Certificate

	EnqueueJob(*Job) (*Job, error)
	ClaimJob(worker string) *Job
	UpdateJob(*Job)
	GetJob(string) *Job
	ListJobs(state string, limit int) []Job

	AppendAudit(AuditEvent)
	ListAudit(limit int) []AuditEvent

	PutToken(*APIToken)
	TokenByHash([]byte) *APIToken
	ListTokens(userID string) []APIToken

	PutBackup(*BackupRun)
	GetBackup(string) *BackupRun
	ListBackups(accountID string) []BackupRun

	PutCron(*CronJob)
	ListCrons(accountID string) []CronJob
	PutSSH(*SSHKey)
	ListSSH(accountID string) []SSHKey
	PutFTP(*FTPAccount)
	ListFTP(accountID string) []FTPAccount
	ListAllFTP() []FTPAccount
	FTPUsernameTaken(username, exceptID string) bool
	DeleteFTP(id string)
	PutUsage(*Usage)
	GetUsage(accountID string) *Usage

	Stats() map[string]int
}
