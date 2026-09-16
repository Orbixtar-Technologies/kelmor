package store

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (p *PG) ImportAccountWithJob(imported *AccountImport, job *Job) (*Job, error) {
	return p.importAccountWithJob(imported, job, nil)
}

func (p *PG) ImportAccountWithJobAndAudit(imported *AccountImport, job *Job, audit AuditEvent) (*Job, error) {
	return p.importAccountWithJob(imported, job, &audit)
}

func (p *PG) importAccountWithJob(imported *AccountImport, job *Job, audit *AuditEvent) (*Job, error) {
	if imported == nil || job == nil {
		return nil, fmt.Errorf("import and job are required")
	}
	if job.TargetRevision == 0 {
		job.TargetRevision = imported.Account.DesiredRevision
	}
	normalizeJob(job)
	payload, err := json.Marshal(job.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal import job payload: %w", err)
	}
	ctx := p.ctx()
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin account import: %w", err)
	}
	defer tx.Rollback(ctx)

	if job.IdempotencyKey != "" {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, job.IdempotencyKey); err != nil {
			return nil, fmt.Errorf("lock account import idempotency key: %w", err)
		}
		existing, scanErr := scanJobRow(tx.QueryRow(ctx, jobSelect+" WHERE idempotency_key=$1 FOR UPDATE", job.IdempotencyKey))
		if scanErr == nil {
			return existing, nil
		}
		if !errors.Is(scanErr, pgx.ErrNoRows) {
			return nil, fmt.Errorf("lookup account import replay: %w", scanErr)
		}
	}

	account := &imported.Account
	if account.LinuxUID < 20000 {
		if err := tx.QueryRow(ctx, `
			UPDATE id_allocators SET next_value=next_value+1
			WHERE name='linux_uid' RETURNING next_value-1`).Scan(&account.LinuxUID); err != nil {
			return nil, fmt.Errorf("allocate imported account UID: %w", err)
		}
		account.LinuxGID = account.LinuxUID
	}
	if err := validateAccountReferences(ctx, tx, account); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO accounts (id, reseller_id, owner_user_id, username, primary_domain, linux_uid, linux_gid, package_id, status, home_path, ip_address, shell_class, login_disabled, desired_revision, observed_revision)
		VALUES ($1,NULLIF($2,'')::uuid,$3,$4,$5,$6,$7,$8,$9,$10,NULLIF($11,'')::inet,$12,$13,$14,$15)`,
		account.ID, account.ResellerID, account.OwnerUserID, account.Username, account.PrimaryDomain,
		account.LinuxUID, account.LinuxGID, account.PackageID, account.Status, account.HomePath,
		account.IPAddress, account.ShellClass, account.LoginDisabled, account.DesiredRevision,
		account.ObservedRevision); err != nil {
		return nil, fmt.Errorf("insert imported account: %w", err)
	}
	for i := range imported.Domains {
		d := &imported.Domains[i]
		if _, err := tx.Exec(ctx, `
			INSERT INTO domains (id, account_id, fqdn, ascii_fqdn, type, document_root, dns_managed, status, desired_revision, observed_revision)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			d.ID, d.AccountID, d.FQDN, d.ASCII, d.Type, d.DocumentRoot, d.DNSManaged,
			d.Status, d.DesiredRevision, d.ObservedRevision); err != nil {
			return nil, fmt.Errorf("insert imported domain %q: %w", d.ID, err)
		}
	}
	for i := range imported.Websites {
		w := &imported.Websites[i]
		if _, err := tx.Exec(ctx, `
			INSERT INTO websites (id, account_id, domain_id, runtime, runtime_version, document_root, https_redirect, www_redirect, proxy_target, enabled, desired_revision, observed_revision)
			VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,COALESCE(NULLIF($8,''),'none'),NULLIF($9,''),$10,$11,$12)`,
			w.ID, w.AccountID, w.DomainID, w.Runtime, w.RuntimeVersion, w.DocumentRoot,
			w.HTTPSRedirect, w.WWWRedirect, w.ProxyTarget, w.Enabled, w.DesiredRevision,
			w.ObservedRevision); err != nil {
			return nil, fmt.Errorf("insert imported website %q: %w", w.ID, err)
		}
	}
	for i := range imported.Applications {
		a := &imported.Applications[i]
		if _, err := tx.Exec(ctx, `
			INSERT INTO applications (id, website_id, account_id, runtime, runtime_version, working_directory, start_command, listen_target, status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			a.ID, a.WebsiteID, a.AccountID, a.Runtime, a.RuntimeVersion, a.WorkingDirectory,
			a.StartCommand, a.ListenTarget, a.Status); err != nil {
			return nil, fmt.Errorf("insert imported application %q: %w", a.ID, err)
		}
	}
	for i := range imported.Databases {
		d := &imported.Databases[i]
		if _, err := tx.Exec(ctx, `
			INSERT INTO hosted_databases (id, account_id, server_id, name, engine, status)
			VALUES ($1,$2,(SELECT id FROM database_servers WHERE engine=$4 ORDER BY id LIMIT 1),$3,$4,$5)`,
			d.ID, d.AccountID, d.Name, d.Engine, d.Status); err != nil {
			return nil, fmt.Errorf("insert imported database %q: %w", d.ID, err)
		}
	}
	for i := range imported.DatabaseUsers {
		u := &imported.DatabaseUsers[i]
		password := u.PasswordEnc
		if password == nil {
			password = []byte{}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO database_users (id, account_id, username, engine, password_enc)
			VALUES ($1,$2,$3,$4,$5)`, u.ID, u.AccountID, u.Username, u.Engine, password); err != nil {
			return nil, fmt.Errorf("insert imported database user %q: %w", u.ID, err)
		}
	}
	for i := range imported.MailDomains {
		d := &imported.MailDomains[i]
		if _, err := tx.Exec(ctx, `
			INSERT INTO mail_domains (id, account_id, domain_id, catchall_policy, status)
			VALUES ($1,$2,$3,$4,$5)`,
			d.ID, d.AccountID, d.DomainID, d.CatchallPolicy, d.Status); err != nil {
			return nil, fmt.Errorf("insert imported mail domain %q: %w", d.ID, err)
		}
	}
	for i := range imported.Mailboxes {
		mb := &imported.Mailboxes[i]
		password := mb.PasswordHash
		if password == "" {
			password = "!"
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO mailboxes (id, account_id, domain_id, local_part, quota_bytes, password_hash, status)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			mb.ID, mb.AccountID, mb.DomainID, mb.LocalPart, mb.QuotaBytes, password, mb.Status); err != nil {
			return nil, fmt.Errorf("insert imported mailbox %q: %w", mb.ID, err)
		}
	}
	for i := range imported.Aliases {
		a := &imported.Aliases[i]
		if _, err := tx.Exec(ctx, `
			INSERT INTO mail_aliases (id, account_id, domain_id, address, destination)
			VALUES ($1,$2,$3,$4,$5)`,
			a.ID, a.AccountID, a.DomainID, a.Address, a.Destination); err != nil {
			return nil, fmt.Errorf("insert imported mail alias %q: %w", a.ID, err)
		}
	}
	for i := range imported.Zones {
		z := &imported.Zones[i]
		if _, err := tx.Exec(ctx, `
			INSERT INTO dns_zones (id, account_id, domain_id, name, dnssec_enabled, provider, desired_revision, observed_revision)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			z.ID, z.AccountID, z.DomainID, z.Name, z.DNSSECEnabled, z.Provider,
			z.DesiredRevision, z.ObservedRevision); err != nil {
			return nil, fmt.Errorf("insert imported DNS zone %q: %w", z.ID, err)
		}
	}
	for i := range imported.Records {
		r := &imported.Records[i]
		if _, err := tx.Exec(ctx, `
			INSERT INTO dns_records (id, zone_id, name, type, content, ttl, priority)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			r.ID, r.ZoneID, r.Name, r.Type, r.Content, r.TTL, r.Priority); err != nil {
			return nil, fmt.Errorf("insert imported DNS record %q: %w", r.ID, err)
		}
	}
	for i := range imported.Crons {
		c := &imported.Crons[i]
		if _, err := tx.Exec(ctx, `
			INSERT INTO cron_jobs (id, account_id, schedule, command, working_directory, enabled)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			c.ID, c.AccountID, c.Schedule, c.Command, c.WorkingDirectory, c.Enabled); err != nil {
			return nil, fmt.Errorf("insert imported cron %q: %w", c.ID, err)
		}
	}
	for i := range imported.SSH {
		k := &imported.SSH[i]
		if _, err := tx.Exec(ctx, `
			INSERT INTO ssh_keys (id, account_id, label, public_key, fingerprint, created_at)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			k.ID, k.AccountID, k.Label, k.PublicKey, k.Fingerprint, k.CreatedAt); err != nil {
			return nil, fmt.Errorf("insert imported SSH key %q: %w", k.ID, err)
		}
	}
	for i := range imported.FTP {
		f := &imported.FTP[i]
		password := f.PasswordHash
		if password == "" {
			password = "!"
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO ftp_accounts (id, account_id, username, home_path, password_hash, status)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			f.ID, f.AccountID, f.Username, f.HomePath, password, f.Status); err != nil {
			return nil, fmt.Errorf("insert imported FTP account %q: %w", f.ID, err)
		}
	}
	if err := insertJobTx(ctx, tx, job, payload); err != nil {
		return nil, fmt.Errorf("insert account import job: %w", err)
	}
	if audit != nil {
		normalizeAudit(audit, job)
		if err := insertAuditTx(ctx, tx, *audit); err != nil {
			return nil, fmt.Errorf("insert account import audit: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit account import: %w", err)
	}
	copy := *job
	return &copy, nil
}
