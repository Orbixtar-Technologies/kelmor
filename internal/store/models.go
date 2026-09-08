package store

import "time"

type User struct {
	ID                 string    `json:"id"`
	Username           string    `json:"username"`
	Email              string    `json:"email"`
	PasswordHash       string    `json:"-"`
	DisplayName        string    `json:"display_name"`
	Status             string    `json:"status"`
	TOTPEnabled        bool      `json:"totp_enabled"`
	MustChangePassword bool      `json:"must_change_password"`
	Roles              []string  `json:"roles"`
	CreatedAt          time.Time `json:"created_at"`
}

type Session struct {
	ID                  string     `json:"id"`
	UserID              string     `json:"user_id"`
	TokenHash           []byte     `json:"-"`
	ExpiresAt           time.Time  `json:"expires_at"`
	RevokedAt           *time.Time `json:"revoked_at,omitempty"`
	SourceIP            string     `json:"source_ip"`
	UserAgent           string     `json:"user_agent"`
	ImpersonatorID      string     `json:"impersonator_id,omitempty"`
	ImpersonationReason string     `json:"impersonation_reason,omitempty"`
}

type FeatureSet struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Features map[string]bool `json:"features"`
}

type Package struct {
	ID                    string `json:"id"`
	ResellerID            string `json:"reseller_id,omitempty"`
	Name                  string `json:"name"`
	FeatureSetID          string `json:"feature_set_id"`
	DiskBytes             int64  `json:"disk_bytes"`
	BandwidthBytesMonthly int64  `json:"bandwidth_bytes_monthly"`
	Domains               int    `json:"domains"`
	Subdomains            int    `json:"subdomains"`
	AliasDomains          int    `json:"alias_domains"`
	Databases             int    `json:"databases"`
	DatabaseUsers         int    `json:"database_users"`
	Mailboxes             int    `json:"mailboxes"`
	MailboxStorageBytes   int64  `json:"mailbox_storage_bytes"`
	FTPUsers              int    `json:"ftp_users"`
	CronJobs              int    `json:"cron_jobs"`
	ApplicationInstances  int    `json:"application_instances"`
	BackupRetentionDays   int    `json:"backup_retention_days"`
	CPUPercent            int    `json:"cpu_percent"`
	MemoryBytes           int64  `json:"memory_bytes"`
	ProcessLimit          int    `json:"process_limit"`
	IOWeight              int    `json:"io_weight"`
	IOPS                  int    `json:"iops"`
	ConcurrentWebRequests int    `json:"concurrent_web_requests"`
	EmailDailyLimit       int    `json:"email_daily_limit"`
}

type Reseller struct {
	ID            string   `json:"id"`
	UserID        string   `json:"user_id"`
	Name          string   `json:"name"`
	BrandName     string   `json:"brand_name,omitempty"`
	PrivilegeMask []string `json:"privilege_mask"`
	Nameservers   []string `json:"nameservers"`
	Status        string   `json:"status"`
}

type Account struct {
	ID               string `json:"id"`
	ResellerID       string `json:"reseller_id,omitempty"`
	OwnerUserID      string `json:"owner_user_id"`
	Username         string `json:"username"`
	PrimaryDomain    string `json:"primary_domain"`
	LinuxUID         int    `json:"linux_uid"`
	LinuxGID         int    `json:"linux_gid"`
	PackageID        string `json:"package_id"`
	Status           string `json:"status"`
	HomePath         string `json:"home_path"`
	IPAddress        string `json:"ip_address,omitempty"`
	ShellClass       string `json:"shell_class"`
	LoginDisabled    bool   `json:"login_disabled"`
	DesiredRevision  int64  `json:"desired_revision"`
	ObservedRevision int64  `json:"observed_revision"`
}

type Domain struct {
	ID           string `json:"id"`
	AccountID    string `json:"account_id"`
	FQDN         string `json:"fqdn"`
	ASCII        string `json:"ascii_fqdn"`
	Type         string `json:"type"`
	DocumentRoot string `json:"document_root,omitempty"`
	DNSManaged   bool   `json:"dns_managed"`
	Status       string `json:"status"`
}

type Website struct {
	ID               string `json:"id"`
	AccountID        string `json:"account_id"`
	DomainID         string `json:"domain_id"`
	Runtime          string `json:"runtime"`
	RuntimeVersion   string `json:"runtime_version,omitempty"`
	DocumentRoot     string `json:"document_root"`
	HTTPSRedirect    bool   `json:"https_redirect"`
	WWWRedirect      string `json:"www_redirect"`
	ProxyTarget      string `json:"proxy_target,omitempty"`
	Enabled          bool   `json:"enabled"`
	DesiredRevision  int64  `json:"desired_revision"`
	ObservedRevision int64  `json:"observed_revision"`
}

type Application struct {
	ID               string `json:"id"`
	WebsiteID        string `json:"website_id"`
	AccountID        string `json:"account_id"`
	Runtime          string `json:"runtime"`
	RuntimeVersion   string `json:"runtime_version"`
	WorkingDirectory string `json:"working_directory"`
	StartCommand     string `json:"start_command"`
	ListenTarget     string `json:"listen_target"`
	Status           string `json:"status"`
}

type HostedDatabase struct {
	ID        string `json:"id"`
	AccountID string `json:"account_id"`
	Engine    string `json:"engine"`
	Name      string `json:"name"`
	Status    string `json:"status"`
}

type DatabaseUser struct {
	ID        string `json:"id"`
	AccountID string `json:"account_id"`
	Username  string `json:"username"`
	Engine    string `json:"engine"`
}

type DNSZone struct {
	ID               string `json:"id"`
	AccountID        string `json:"account_id"`
	DomainID         string `json:"domain_id"`
	Name             string `json:"name"`
	DNSSECEnabled    bool   `json:"dnssec_enabled"`
	Provider         string `json:"provider"`
	DesiredRevision  int64  `json:"desired_revision"`
	ObservedRevision int64  `json:"observed_revision"`
}

type DNSRecord struct {
	ID       string `json:"id"`
	ZoneID   string `json:"zone_id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"`
	Priority *int   `json:"priority,omitempty"`
}

type MailDomain struct {
	ID             string `json:"id"`
	AccountID      string `json:"account_id"`
	DomainID       string `json:"domain_id"`
	CatchallPolicy string `json:"catchall_policy"`
	Status         string `json:"status"`
}

type Mailbox struct {
	ID           string `json:"id"`
	AccountID    string `json:"account_id"`
	DomainID     string `json:"domain_id"`
	LocalPart    string `json:"local_part"`
	QuotaBytes   int64  `json:"quota_bytes"`
	PasswordHash string `json:"-"`
	Status       string `json:"status"`
}

type MailAlias struct {
	ID          string `json:"id"`
	AccountID   string `json:"account_id"`
	DomainID    string `json:"domain_id"`
	Address     string `json:"address"`
	Destination string `json:"destination"`
}

type Certificate struct {
	ID        string     `json:"id"`
	AccountID string     `json:"account_id,omitempty"`
	Hostname  string     `json:"hostname"`
	Kind      string     `json:"kind"`
	Status    string     `json:"status"`
	NotAfter  *time.Time `json:"not_after,omitempty"`
	Issuer    string     `json:"issuer,omitempty"`
}

type Job struct {
	ID             string         `json:"id"`
	Type           string         `json:"type"`
	ResourceType   string         `json:"resource_type,omitempty"`
	ResourceID     string         `json:"resource_id,omitempty"`
	Payload        map[string]any `json:"payload"`
	State          string         `json:"state"`
	Priority       int            `json:"priority"`
	Attempts       int            `json:"attempts"`
	MaxAttempts    int            `json:"max_attempts"`
	Progress       int            `json:"progress"`
	RunAfter       time.Time      `json:"run_after"`
	LockedBy       string         `json:"locked_by,omitempty"`
	HeartbeatAt    *time.Time     `json:"heartbeat_at,omitempty"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	LastError      string         `json:"last_error,omitempty"`
	ActorID        string         `json:"actor_id,omitempty"`
	RequestID      string         `json:"request_id,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	StartedAt      *time.Time     `json:"started_at,omitempty"`
	FinishedAt     *time.Time     `json:"finished_at,omitempty"`
	Logs           []string       `json:"logs,omitempty"`
}

type AuditEvent struct {
	ID             string         `json:"id"`
	OccurredAt     time.Time      `json:"occurred_at"`
	ActorType      string         `json:"actor_type"`
	ActorID        string         `json:"actor_id,omitempty"`
	EffectiveActor string         `json:"effective_actor_id,omitempty"`
	AccountID      string         `json:"account_id,omitempty"`
	Action         string         `json:"action"`
	ResourceType   string         `json:"resource_type,omitempty"`
	ResourceID     string         `json:"resource_id,omitempty"`
	SourceIP       string         `json:"source_ip,omitempty"`
	UserAgent      string         `json:"user_agent,omitempty"`
	RequestID      string         `json:"request_id"`
	Success        bool           `json:"success"`
	Before         map[string]any `json:"before_state,omitempty"`
	After          map[string]any `json:"after_state,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type APIToken struct {
	ID           string     `json:"id"`
	UserID       string     `json:"user_id"`
	Name         string     `json:"name"`
	Prefix       string     `json:"prefix"`
	TokenHash    []byte     `json:"-"`
	Scope        string     `json:"scope"`
	AccountID    string     `json:"account_id,omitempty"`
	Capabilities []string   `json:"capabilities"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
}

type BackupRun struct {
	ID          string         `json:"id"`
	AccountID   string         `json:"account_id"`
	Kind        string         `json:"kind"`
	State       string         `json:"state"`
	Destination string         `json:"destination"`
	Checksum    string         `json:"checksum,omitempty"`
	SizeBytes   int64          `json:"size_bytes"`
	CreatedAt   time.Time      `json:"created_at"`
	FinishedAt  *time.Time     `json:"finished_at,omitempty"`
	Manifest    map[string]any `json:"manifest,omitempty"`
}

type CronJob struct {
	ID               string `json:"id"`
	AccountID        string `json:"account_id"`
	Schedule         string `json:"schedule"`
	Command          string `json:"command"`
	WorkingDirectory string `json:"working_directory"`
	Enabled          bool   `json:"enabled"`
}

type SSHKey struct {
	ID          string    `json:"id"`
	AccountID   string    `json:"account_id"`
	Label       string    `json:"label"`
	PublicKey   string    `json:"public_key"`
	Fingerprint string    `json:"fingerprint"`
	CreatedAt   time.Time `json:"created_at"`
}

type FTPAccount struct {
	ID           string `json:"id"`
	AccountID    string `json:"account_id"`
	Username     string `json:"username"`
	HomePath     string `json:"home_path"`
	PasswordHash string `json:"-"`
	Status       string `json:"status"`
}

type Usage struct {
	AccountID      string    `json:"account_id"`
	CollectedAt    time.Time `json:"collected_at"`
	DiskBytes      int64     `json:"disk_bytes"`
	InodeCount     int64     `json:"inode_count"`
	BandwidthBytes int64     `json:"bandwidth_bytes"`
	CPUPercent     float64   `json:"cpu_percent"`
	MemoryBytes    int64     `json:"memory_bytes"`
	ProcessCount   int       `json:"process_count"`
}
