package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (m *Memory) commitAsyncMutationLocked(job *Job, audit AuditEvent, validate func() error, mutate func()) (*Job, error) {
	if job == nil {
		return nil, fmt.Errorf("job is required")
	}
	if replay := m.jobReplayLocked(job); replay != nil {
		return replay, nil
	}
	if err := validate(); err != nil {
		return nil, err
	}
	normalizeJob(job)
	if err := m.validateNewJobLocked(job); err != nil {
		return nil, err
	}
	normalizeAudit(&audit, job)
	mutate()
	jobCopy := *job
	auditCopy := audit
	m.Jobs[job.ID] = &jobCopy
	m.Audit = append(m.Audit, &auditCopy)
	return &jobCopy, nil
}

func (m *Memory) UpsertApplicationWithJob(application *Application, job *Job, audit AuditEvent) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.commitAsyncMutationLocked(job, audit, func() error {
		if application == nil {
			return fmt.Errorf("application is required")
		}
		website := m.Websites[application.WebsiteID]
		if website == nil || website.AccountID != application.AccountID {
			return fmt.Errorf("website %q is not owned by account %q", application.WebsiteID, application.AccountID)
		}
		return nil
	}, func() {
		copy := *application
		m.Apps[application.ID] = &copy
	})
}

func (m *Memory) UpsertDatabaseWithJob(database *HostedDatabase, user *DatabaseUser, job *Job, audit AuditEvent) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.commitAsyncMutationLocked(job, audit, func() error {
		if database == nil || m.Accounts[database.AccountID] == nil {
			return fmt.Errorf("database account is missing")
		}
		if user != nil && user.AccountID != database.AccountID {
			return fmt.Errorf("database user belongs to another account")
		}
		return nil
	}, func() {
		copy := *database
		m.DBs[database.ID] = &copy
		if user != nil {
			userCopy := *user
			m.DBUsers[user.ID] = &userCopy
		}
	})
}

func (m *Memory) DeleteDatabaseWithJob(databaseID, accountID string, job *Job, audit AuditEvent) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.commitAsyncMutationLocked(job, audit, func() error {
		database := m.DBs[databaseID]
		if database == nil || database.AccountID != accountID {
			return fmt.Errorf("database %q not found", databaseID)
		}
		return nil
	}, func() {
		copy := *m.DBs[databaseID]
		copy.Status = "terminating"
		m.DBs[databaseID] = &copy
	})
}

func (m *Memory) UpsertZoneWithJob(zone *DNSZone, job *Job, audit AuditEvent) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.commitAsyncMutationLocked(job, audit, func() error {
		if zone == nil || m.Accounts[zone.AccountID] == nil {
			return fmt.Errorf("DNS zone account is missing")
		}
		return nil
	}, func() {
		copy := *zone
		m.Zones[zone.ID] = &copy
	})
}

func (m *Memory) CreateRecordWithJob(record *DNSRecord, accountID string, job *Job, audit AuditEvent) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.commitAsyncMutationLocked(job, audit, func() error {
		if record == nil {
			return fmt.Errorf("DNS record is required")
		}
		zone := m.Zones[record.ZoneID]
		if zone == nil || zone.AccountID != accountID {
			return fmt.Errorf("DNS zone %q not found", record.ZoneID)
		}
		return nil
	}, func() {
		copy := *record
		m.Records[record.ID] = &copy
	})
}

func (m *Memory) DeleteRecordWithJob(recordID, zoneID, accountID string, job *Job, audit AuditEvent) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.commitAsyncMutationLocked(job, audit, func() error {
		zone := m.Zones[zoneID]
		record := m.Records[recordID]
		if zone == nil || zone.AccountID != accountID || record == nil || record.ZoneID != zoneID {
			return fmt.Errorf("DNS record %q not found", recordID)
		}
		return nil
	}, func() { delete(m.Records, recordID) })
}

func (m *Memory) UpsertMailDomainWithJob(domain *MailDomain, job *Job, audit AuditEvent) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.commitAsyncMutationLocked(job, audit, func() error {
		if domain == nil || m.Accounts[domain.AccountID] == nil {
			return fmt.Errorf("mail domain account is missing")
		}
		return nil
	}, func() {
		copy := *domain
		m.MailDom[domain.ID] = &copy
	})
}

func (m *Memory) UpsertMailboxWithJob(mailbox *Mailbox, job *Job, audit AuditEvent) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.commitAsyncMutationLocked(job, audit, func() error {
		if mailbox == nil {
			return fmt.Errorf("mailbox is required")
		}
		domain := m.MailDom[mailbox.DomainID]
		if domain == nil || domain.AccountID != mailbox.AccountID {
			return fmt.Errorf("mail domain %q not found", mailbox.DomainID)
		}
		return nil
	}, func() {
		copy := *mailbox
		m.Mailboxes[mailbox.ID] = &copy
	})
}

func (m *Memory) DeleteMailboxWithJob(mailboxID, accountID string, job *Job, audit AuditEvent) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.commitAsyncMutationLocked(job, audit, func() error {
		mailbox := m.Mailboxes[mailboxID]
		if mailbox == nil || mailbox.AccountID != accountID {
			return fmt.Errorf("mailbox %q not found", mailboxID)
		}
		return nil
	}, func() { delete(m.Mailboxes, mailboxID) })
}

func (m *Memory) UpsertMailAliasWithJob(alias *MailAlias, job *Job, audit AuditEvent) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.commitAsyncMutationLocked(job, audit, func() error {
		if alias == nil {
			return fmt.Errorf("mail alias is required")
		}
		domain := m.MailDom[alias.DomainID]
		if domain == nil || domain.AccountID != alias.AccountID {
			return fmt.Errorf("mail domain %q not found", alias.DomainID)
		}
		return nil
	}, func() {
		copy := *alias
		m.Aliases[alias.ID] = &copy
	})
}

func (m *Memory) DeleteMailAliasWithJob(aliasID, accountID string, job *Job, audit AuditEvent) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.commitAsyncMutationLocked(job, audit, func() error {
		alias := m.Aliases[aliasID]
		if alias == nil || alias.AccountID != accountID {
			return fmt.Errorf("mail alias %q not found", aliasID)
		}
		return nil
	}, func() { delete(m.Aliases, aliasID) })
}

func (m *Memory) UpsertCronWithJob(cron *CronJob, job *Job, audit AuditEvent) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.commitAsyncMutationLocked(job, audit, func() error {
		if cron == nil || m.Accounts[cron.AccountID] == nil {
			return fmt.Errorf("cron account is missing")
		}
		return nil
	}, func() {
		copy := *cron
		m.Crons[cron.ID] = &copy
	})
}

func (m *Memory) DeleteCronWithJob(cronID, accountID string, job *Job, audit AuditEvent) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.commitAsyncMutationLocked(job, audit, func() error {
		cron := m.Crons[cronID]
		if cron == nil || cron.AccountID != accountID {
			return fmt.Errorf("cron %q not found", cronID)
		}
		return nil
	}, func() { delete(m.Crons, cronID) })
}

func (m *Memory) UpsertFTPWithJob(ftp *FTPAccount, job *Job, audit AuditEvent) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.commitAsyncMutationLocked(job, audit, func() error {
		if ftp == nil || m.Accounts[ftp.AccountID] == nil {
			return fmt.Errorf("FTP account owner is missing")
		}
		for id, existing := range m.FTPs {
			if id != ftp.ID && existing.Username == ftp.Username {
				return fmt.Errorf("FTP username %q already exists", ftp.Username)
			}
		}
		return nil
	}, func() {
		copy := *ftp
		m.FTPs[ftp.ID] = &copy
	})
}

func (m *Memory) DeleteFTPWithJob(ftpID, accountID string, job *Job, audit AuditEvent) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.commitAsyncMutationLocked(job, audit, func() error {
		ftp := m.FTPs[ftpID]
		if ftp == nil || ftp.AccountID != accountID {
			return fmt.Errorf("FTP account %q not found", ftpID)
		}
		return nil
	}, func() { delete(m.FTPs, ftpID) })
}

func (p *PG) UpsertApplicationWithJob(application *Application, job *Job, audit AuditEvent) (*Job, error) {
	if application == nil {
		return nil, fmt.Errorf("application is required")
	}
	return p.commitResourceMutation(job, audit, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO applications (id, website_id, account_id, runtime, runtime_version, working_directory, start_command, listen_target, status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (id) DO UPDATE SET status=EXCLUDED.status, start_command=EXCLUDED.start_command`,
			application.ID, application.WebsiteID, application.AccountID, application.Runtime,
			application.RuntimeVersion, application.WorkingDirectory, application.StartCommand,
			application.ListenTarget, application.Status)
		return err
	})
}

func (p *PG) UpsertDatabaseWithJob(database *HostedDatabase, user *DatabaseUser, job *Job, audit AuditEvent) (*Job, error) {
	if database == nil {
		return nil, fmt.Errorf("database is required")
	}
	return p.commitResourceMutation(job, audit, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO hosted_databases (id, account_id, server_id, name, engine, status)
			VALUES ($1,$2,(SELECT id FROM database_servers WHERE engine=$4 ORDER BY id LIMIT 1),$3,$4,$5)
			ON CONFLICT (id) DO UPDATE SET status=EXCLUDED.status`,
			database.ID, database.AccountID, database.Name, database.Engine, database.Status); err != nil {
			return err
		}
		if user == nil {
			return nil
		}
		password := user.PasswordEnc
		if password == nil {
			password = []byte{}
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO database_users (id, account_id, username, engine, password_enc)
			VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (account_id, username, engine) DO NOTHING`,
			user.ID, user.AccountID, user.Username, user.Engine, password)
		return err
	})
}

func (p *PG) DeleteDatabaseWithJob(databaseID, accountID string, job *Job, audit AuditEvent) (*Job, error) {
	return p.commitResourceMutation(job, audit, func(ctx context.Context, tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE hosted_databases SET status='terminating'
			WHERE id=$1 AND account_id=$2`, databaseID, accountID)
		if err == nil && result.RowsAffected() != 1 {
			return fmt.Errorf("database %q not found", databaseID)
		}
		return err
	})
}

func (p *PG) UpsertZoneWithJob(zone *DNSZone, job *Job, audit AuditEvent) (*Job, error) {
	if zone == nil {
		return nil, fmt.Errorf("DNS zone is required")
	}
	return p.commitResourceMutation(job, audit, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO dns_zones (id, account_id, domain_id, name, dnssec_enabled, provider, desired_revision, observed_revision)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT (id) DO UPDATE SET dnssec_enabled=EXCLUDED.dnssec_enabled,
				desired_revision=EXCLUDED.desired_revision`,
			zone.ID, zone.AccountID, zone.DomainID, zone.Name, zone.DNSSECEnabled,
			zone.Provider, zone.DesiredRevision, zone.ObservedRevision)
		return err
	})
}

func (p *PG) CreateRecordWithJob(record *DNSRecord, accountID string, job *Job, audit AuditEvent) (*Job, error) {
	if record == nil {
		return nil, fmt.Errorf("DNS record is required")
	}
	return p.commitResourceMutation(job, audit, func(ctx context.Context, tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			INSERT INTO dns_records (id, zone_id, name, type, content, ttl, priority)
			SELECT $1,$2,$3,$4,$5,$6,$7
			WHERE EXISTS (SELECT 1 FROM dns_zones WHERE id=$2 AND account_id=$8)`,
			record.ID, record.ZoneID, record.Name, record.Type, record.Content,
			record.TTL, record.Priority, accountID)
		if err == nil && result.RowsAffected() != 1 {
			return fmt.Errorf("DNS zone %q not found", record.ZoneID)
		}
		return err
	})
}

func (p *PG) DeleteRecordWithJob(recordID, zoneID, accountID string, job *Job, audit AuditEvent) (*Job, error) {
	return p.commitResourceMutation(job, audit, func(ctx context.Context, tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			DELETE FROM dns_records
			WHERE id=$1 AND zone_id=$2
			  AND EXISTS (SELECT 1 FROM dns_zones WHERE id=$2 AND account_id=$3)`,
			recordID, zoneID, accountID)
		if err == nil && result.RowsAffected() != 1 {
			return fmt.Errorf("DNS record %q not found", recordID)
		}
		return err
	})
}

func (p *PG) UpsertMailDomainWithJob(domain *MailDomain, job *Job, audit AuditEvent) (*Job, error) {
	if domain == nil {
		return nil, fmt.Errorf("mail domain is required")
	}
	return p.commitResourceMutation(job, audit, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO mail_domains (id, account_id, domain_id, catchall_policy, status)
			VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (id) DO UPDATE SET catchall_policy=EXCLUDED.catchall_policy, status=EXCLUDED.status`,
			domain.ID, domain.AccountID, domain.DomainID, domain.CatchallPolicy, domain.Status)
		return err
	})
}

func (p *PG) UpsertMailboxWithJob(mailbox *Mailbox, job *Job, audit AuditEvent) (*Job, error) {
	if mailbox == nil {
		return nil, fmt.Errorf("mailbox is required")
	}
	return p.commitResourceMutation(job, audit, func(ctx context.Context, tx pgx.Tx) error {
		password := mailbox.PasswordHash
		if password == "" {
			password = "!"
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO mailboxes (id, account_id, domain_id, local_part, quota_bytes, password_hash, status)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (id) DO UPDATE SET quota_bytes=EXCLUDED.quota_bytes,
				password_hash=EXCLUDED.password_hash, status=EXCLUDED.status`,
			mailbox.ID, mailbox.AccountID, mailbox.DomainID, mailbox.LocalPart,
			mailbox.QuotaBytes, password, mailbox.Status)
		return err
	})
}

func (p *PG) DeleteMailboxWithJob(mailboxID, accountID string, job *Job, audit AuditEvent) (*Job, error) {
	return p.commitResourceMutation(job, audit, func(ctx context.Context, tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `DELETE FROM mailboxes WHERE id=$1 AND account_id=$2`, mailboxID, accountID)
		if err == nil && result.RowsAffected() != 1 {
			return fmt.Errorf("mailbox %q not found", mailboxID)
		}
		return err
	})
}

func (p *PG) UpsertMailAliasWithJob(alias *MailAlias, job *Job, audit AuditEvent) (*Job, error) {
	if alias == nil {
		return nil, fmt.Errorf("mail alias is required")
	}
	return p.commitResourceMutation(job, audit, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO mail_aliases (id, account_id, domain_id, address, destination)
			VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (id) DO UPDATE SET address=EXCLUDED.address, destination=EXCLUDED.destination`,
			alias.ID, alias.AccountID, alias.DomainID, alias.Address, alias.Destination)
		return err
	})
}

func (p *PG) DeleteMailAliasWithJob(aliasID, accountID string, job *Job, audit AuditEvent) (*Job, error) {
	return p.commitResourceMutation(job, audit, func(ctx context.Context, tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `DELETE FROM mail_aliases WHERE id=$1 AND account_id=$2`, aliasID, accountID)
		if err == nil && result.RowsAffected() != 1 {
			return fmt.Errorf("mail alias %q not found", aliasID)
		}
		return err
	})
}

func (p *PG) UpsertCronWithJob(cron *CronJob, job *Job, audit AuditEvent) (*Job, error) {
	if cron == nil {
		return nil, fmt.Errorf("cron is required")
	}
	return p.commitResourceMutation(job, audit, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO cron_jobs (id, account_id, schedule, command, working_directory, enabled)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (id) DO UPDATE SET schedule=EXCLUDED.schedule, command=EXCLUDED.command,
				working_directory=EXCLUDED.working_directory, enabled=EXCLUDED.enabled`,
			cron.ID, cron.AccountID, cron.Schedule, cron.Command, cron.WorkingDirectory, cron.Enabled)
		return err
	})
}

func (p *PG) DeleteCronWithJob(cronID, accountID string, job *Job, audit AuditEvent) (*Job, error) {
	return p.commitResourceMutation(job, audit, func(ctx context.Context, tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `DELETE FROM cron_jobs WHERE id=$1 AND account_id=$2`, cronID, accountID)
		if err == nil && result.RowsAffected() != 1 {
			return fmt.Errorf("cron %q not found", cronID)
		}
		return err
	})
}

func (p *PG) UpsertFTPWithJob(ftp *FTPAccount, job *Job, audit AuditEvent) (*Job, error) {
	if ftp == nil {
		return nil, fmt.Errorf("FTP account is required")
	}
	return p.commitResourceMutation(job, audit, func(ctx context.Context, tx pgx.Tx) error {
		password := ftp.PasswordHash
		if password == "" {
			password = "!"
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO ftp_accounts (id, account_id, username, home_path, password_hash, status)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (id) DO UPDATE SET username=EXCLUDED.username,
				home_path=EXCLUDED.home_path, password_hash=EXCLUDED.password_hash,
				status=EXCLUDED.status`,
			ftp.ID, ftp.AccountID, ftp.Username, ftp.HomePath, password, ftp.Status)
		return err
	})
}

func (p *PG) DeleteFTPWithJob(ftpID, accountID string, job *Job, audit AuditEvent) (*Job, error) {
	return p.commitResourceMutation(job, audit, func(ctx context.Context, tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `DELETE FROM ftp_accounts WHERE id=$1 AND account_id=$2`, ftpID, accountID)
		if err == nil && result.RowsAffected() != 1 {
			return fmt.Errorf("FTP account %q not found", ftpID)
		}
		return err
	})
}
