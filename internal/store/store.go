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
	DeletePackageIfUnused(string) bool

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
	DeleteDomain(id string)

	PutWebsite(*Website)
	GetWebsite(string) *Website
	ListWebsites(accountID string) []Website
	DeleteWebsite(id string)

	PutApp(*Application)
	GetApp(string) *Application
	ListApps(accountID string) []Application
	DeleteApp(id string)

	PutDB(*HostedDatabase)
	GetDB(string) *HostedDatabase
	ListDBs(accountID string) []HostedDatabase
	DeleteDB(id string)
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
	DeleteMailbox(id string)
	PutMailAlias(*MailAlias)
	GetMailAlias(string) *MailAlias
	ListMailAliases(accountID string) []MailAlias
	DeleteMailAlias(id string)

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
	GetToken(id string) *APIToken
	TokenByHash([]byte) *APIToken
	ListTokens(userID string) []APIToken
	DeleteToken(id string)

	PutBackup(*BackupRun)
	GetBackup(string) *BackupRun
	ListBackups(accountID string) []BackupRun

	PutCron(*CronJob)
	GetCron(id string) *CronJob
	ListCrons(accountID string) []CronJob
	DeleteCron(id string)
	PutSSH(*SSHKey)
	GetSSH(id string) *SSHKey
	ListSSH(accountID string) []SSHKey
	DeleteSSH(id string)
	PutFTP(*FTPAccount)
	ListFTP(accountID string) []FTPAccount
	ListAllFTP() []FTPAccount
	FTPUsernameTaken(username, exceptID string) bool
	DeleteFTP(id string)
	PutUsage(*Usage)
	GetUsage(accountID string) *Usage

	Stats() map[string]int
}
