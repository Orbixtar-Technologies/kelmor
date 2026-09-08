package store

import (
	"time"

	"github.com/hosting-panel/panel/internal/auth"
	"github.com/hosting-panel/panel/internal/id"
)

func SeedDev(m *Memory, adminUser, adminPass, adminEmail string) error {
	hash, err := auth.HashPassword(adminPass)
	if err != nil {
		return err
	}
	admin := &User{
		ID: id.New(), Username: adminUser, Email: adminEmail, PasswordHash: hash,
		DisplayName: "Root Owner", Status: "active", Roles: []string{"root_owner"},
		MustChangePassword: false, CreatedAt: time.Now().UTC(),
	}
	m.PutUser(admin)
	fs := &FeatureSet{ID: id.New(), Name: "full-hosting", Features: map[string]bool{
		"websites": true, "dns": true, "email": true, "databases": true, "files": true,
		"ftp": true, "applications": true, "wordpress": true, "ssl": true, "backups": true,
		"cron": true, "ssh": true, "api": true,
	}}
	m.PutFeature(fs)
	pkg := &Package{
		ID: id.New(), Name: "Starter", FeatureSetID: fs.ID,
		DiskBytes: 10 << 30, BandwidthBytesMonthly: 100 << 30,
		Domains: 10, Subdomains: 50, AliasDomains: 20, Databases: 10, DatabaseUsers: 20,
		Mailboxes: 50, MailboxStorageBytes: 5 << 30, FTPUsers: 10, CronJobs: 20,
		ApplicationInstances: 5, BackupRetentionDays: 14,
		CPUPercent: 200, MemoryBytes: 2 << 30, ProcessLimit: 200, IOWeight: 100, IOPS: 1000,
		ConcurrentWebRequests: 200, EmailDailyLimit: 500,
	}
	m.PutPackage(pkg)
	return nil
}
