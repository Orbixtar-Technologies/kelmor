package store

import (
	"errors"
	"time"
)

var (
	ErrStaleAccount      = errors.New("stale account revision")
	ErrStaleFence        = errors.New("stale job fence")
	ErrJobNotFound       = errors.New("job not found")
	ErrJobStateConflict  = errors.New("job state does not allow cancellation")
	ErrRestoreInProgress = errors.New("restore already in progress")
)

const (
	StateRestoreRequested   = "restore_requested"
	StateMaintenance        = "maintenance"
	StateManualIntervention = "manual_intervention"
	StateRestoreComplete    = "complete"
	StateRestoreFailed      = "failed"
)

const JobLeaseTTL = 2 * time.Minute

type AuditFilter struct {
	Query        string
	Action       string
	ResourceType string
	AccountID    string
	ActorID      string
	Success      *bool
	Since        *time.Time
	Until        *time.Time
	Limit        int
	Offset       int
	Cursor       string
}

type Store interface {
	PutUser(*User)
	UserByUsername(string) *User
	UserByID(string) *User

	PutSession(*Session)
	SessionByHash([]byte) *Session
	RevokeSession(string)
	RevokeSessionsForUser(userID string)

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
	CreateAccountWithJob(owner *User, account *Account, domain *Domain, memberUserIDs []string, job *Job) (*Job, error)
	CreateAccountWithJobAndAudit(owner *User, account *Account, domain *Domain, memberUserIDs []string, job *Job, audit AuditEvent) (*Job, error)
	UpdateAccountWithJob(account *Account, job *Job) (*Job, error)
	UpdateAccountWithJobAndAudit(account *Account, job *Job, audit AuditEvent) (*Job, error)
	ImportAccountWithJob(imported *AccountImport, job *Job) (*Job, error)
	ImportAccountWithJobAndAudit(imported *AccountImport, job *Job, audit AuditEvent) (*Job, error)
	GetAccount(string) *Account
	AccountByUsername(string) *Account
	ListAccounts(q, status string) []Account
	ListAccountsPage(q, status, cursor string, limit int) AccountPage
	AddMember(accountID, userID string)
	AccountsForUser(userID string) []string
	DriftedAccounts() []Account

	DomainTaken(ascii string) bool
	PutDomain(*Domain)
	CreateDomainWithJob(domain *Domain, job *Job, audit AuditEvent) (*Job, error)
	UpdateDomainWithJob(domain *Domain, job *Job, audit AuditEvent) (*Job, error)
	GetDomain(string) *Domain
	ListDomains(accountID string) []Domain
	DeleteDomain(id string)

	PutWebsite(*Website)
	UpsertWebsiteWithJob(website *Website, job *Job, audit AuditEvent) (*Job, error)
	GetWebsite(string) *Website
	ListWebsites(accountID string) []Website
	DeleteWebsite(id string)

	PutApp(*Application)
	UpsertApplicationWithJob(application *Application, job *Job, audit AuditEvent) (*Job, error)
	GetApp(string) *Application
	ListApps(accountID string) []Application
	DeleteApp(id string)

	PutDB(*HostedDatabase)
	UpsertDatabaseWithJob(database *HostedDatabase, user *DatabaseUser, job *Job, audit AuditEvent) (*Job, error)
	DeleteDatabaseWithJob(databaseID, accountID string, job *Job, audit AuditEvent) (*Job, error)
	GetDB(string) *HostedDatabase
	ListDBs(accountID string) []HostedDatabase
	DeleteDB(id string)
	PutDBUser(*DatabaseUser)
	ListDBUsers(accountID string) []DatabaseUser

	PutZone(*DNSZone)
	GetZone(string) *DNSZone
	ZoneByDomain(domainID string) *DNSZone
	ListZones(accountID string) []DNSZone
	ListZonesPage(accountID, cursor string, limit int) ZonePage
	PutRecord(*DNSRecord)
	UpsertZoneWithJob(zone *DNSZone, job *Job, audit AuditEvent) (*Job, error)
	CreateRecordWithJob(record *DNSRecord, accountID string, job *Job, audit AuditEvent) (*Job, error)
	DeleteRecordWithJob(recordID, zoneID, accountID string, job *Job, audit AuditEvent) (*Job, error)
	ListRecords(zoneID string) []DNSRecord
	DeleteRecord(id string)

	PutMailDomain(*MailDomain)
	UpsertMailDomainWithJob(domain *MailDomain, job *Job, audit AuditEvent) (*Job, error)
	GetMailDomain(string) *MailDomain
	MailDomainByDomain(domainID string) *MailDomain
	ListMailDomains(accountID string) []MailDomain
	PutMailbox(*Mailbox)
	UpsertMailboxWithJob(mailbox *Mailbox, job *Job, audit AuditEvent) (*Job, error)
	DeleteMailboxWithJob(mailboxID, accountID string, job *Job, audit AuditEvent) (*Job, error)
	GetMailbox(string) *Mailbox
	ListMailboxes(accountID string) []Mailbox
	DeleteMailbox(id string)
	PutMailAlias(*MailAlias)
	UpsertMailAliasWithJob(alias *MailAlias, job *Job, audit AuditEvent) (*Job, error)
	DeleteMailAliasWithJob(aliasID, accountID string, job *Job, audit AuditEvent) (*Job, error)
	GetMailAlias(string) *MailAlias
	ListMailAliases(accountID string) []MailAlias
	DeleteMailAlias(id string)

	PutCert(*Certificate)
	UpsertCertificateWithJob(certificate *Certificate, job *Job, audit AuditEvent) (*Job, error)
	GetCert(string) *Certificate
	ListCerts(accountID string) []Certificate
	ListDueCertificates(cutoff time.Time, limit int) []Certificate

	EnqueueJob(*Job) (*Job, error)
	EnqueueJobWithAudit(job *Job, audit AuditEvent) (*Job, error)
	RotatePasswordAndEnqueue(userID, passwordHash string, mustChange bool, job *Job) (*Job, error)
	ClaimJob(worker string) *Job
	HeartbeatJob(jobID, owner string, fence int64) error
	ExpireStaleLeases(now time.Time) error
	RequestJobCancel(jobID string) error
	UpdateJob(*Job) error
	GetJob(string) *Job
	JobByIdempotencyKey(string) *Job
	ListJobs(state string, limit int) []Job
	CancelJob(jobID, actorID, requestID string) (*Job, error)

	AppendAudit(AuditEvent)
	ListAudit(limit int) []AuditEvent
	QueryAudit(AuditFilter) AuditPage

	PutToken(*APIToken)
	GetToken(id string) *APIToken
	TokenByHash([]byte) *APIToken
	ListTokens(userID string) []APIToken
	DeleteToken(id string)

	PutBackup(*BackupRun)
	CreateBackupWithJob(backup *BackupRun, job *Job, audit AuditEvent) (*Job, error)
	GetBackup(string) *BackupRun
	ListBackups(accountID string) []BackupRun
	ListBackupsPage(accountID string, limit int) []BackupRun
	BumpResourceFence(resourceKey string) int64
	BeginRestore(*RestoreJournal) (*RestoreJournal, error)
	CreateRestoreWithJob(journal *RestoreJournal, job *Job, audit AuditEvent) (*Job, error)
	GetRestore(id string) *RestoreJournal
	GetActiveRestore(accountID string) *RestoreJournal
	CheckpointRestore(id, state string, extra map[string]any) error
	FailRestore(id, state string, manual bool) error
	FinishRestore(id string) error

	PutCron(*CronJob)
	UpsertCronWithJob(cron *CronJob, job *Job, audit AuditEvent) (*Job, error)
	DeleteCronWithJob(cronID, accountID string, job *Job, audit AuditEvent) (*Job, error)
	GetCron(id string) *CronJob
	ListCrons(accountID string) []CronJob
	DeleteCron(id string)
	PutSSH(*SSHKey)
	GetSSH(id string) *SSHKey
	ListSSH(accountID string) []SSHKey
	DeleteSSH(id string)
	PutFTP(*FTPAccount)
	UpsertFTPWithJob(ftp *FTPAccount, job *Job, audit AuditEvent) (*Job, error)
	DeleteFTPWithJob(ftpID, accountID string, job *Job, audit AuditEvent) (*Job, error)
	ListFTP(accountID string) []FTPAccount
	ListAllFTP() []FTPAccount
	FTPUsernameTaken(username, exceptID string) bool
	DeleteFTP(id string)
	PutUsage(*Usage)
	GetUsage(accountID string) *Usage

	Stats() map[string]int
}
