package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hosting-panel/panel/internal/id"
	"github.com/jackc/pgx/v5"
)

func normalizeAudit(event *AuditEvent, job *Job) {
	if event.ID == "" {
		event.ID = id.New()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	if event.RequestID == "" {
		event.RequestID = job.RequestID
	}
}

func (m *Memory) CreateDomainWithJob(domain *Domain, job *Job, audit AuditEvent) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if domain == nil || job == nil {
		return nil, fmt.Errorf("domain and job are required")
	}
	if m.Accounts[domain.AccountID] == nil {
		return nil, fmt.Errorf("account %q not found", domain.AccountID)
	}
	if m.Domains[domain.ID] != nil {
		return nil, fmt.Errorf("domain %q already exists", domain.ID)
	}
	for _, existing := range m.Domains {
		if existing.ASCII == domain.ASCII {
			return nil, fmt.Errorf("domain %q already exists", domain.ASCII)
		}
	}
	return m.commitDomainMutationLocked(domain, job, audit)
}

func (m *Memory) UpdateDomainWithJob(domain *Domain, job *Job, audit AuditEvent) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if domain == nil || job == nil {
		return nil, fmt.Errorf("domain and job are required")
	}
	current := m.Domains[domain.ID]
	if current == nil || current.AccountID != domain.AccountID {
		return nil, fmt.Errorf("domain %q not found", domain.ID)
	}
	return m.commitDomainMutationLocked(domain, job, audit)
}

func (m *Memory) commitDomainMutationLocked(domain *Domain, job *Job, audit AuditEvent) (*Job, error) {
	if replay := m.jobReplayLocked(job); replay != nil {
		return replay, nil
	}
	normalizeJob(job)
	if err := m.validateNewJobLocked(job); err != nil {
		return nil, err
	}
	normalizeAudit(&audit, job)
	domainCopy := *domain
	jobCopy := *job
	auditCopy := audit
	m.Domains[domain.ID] = &domainCopy
	m.Jobs[job.ID] = &jobCopy
	m.Audit = append(m.Audit, &auditCopy)
	return &jobCopy, nil
}

func (m *Memory) UpsertWebsiteWithJob(website *Website, job *Job, audit AuditEvent) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if website == nil || job == nil {
		return nil, fmt.Errorf("website and job are required")
	}
	if m.Accounts[website.AccountID] == nil {
		return nil, fmt.Errorf("account %q not found", website.AccountID)
	}
	domain := m.Domains[website.DomainID]
	if domain == nil || domain.AccountID != website.AccountID {
		return nil, fmt.Errorf("domain %q is not owned by account %q", website.DomainID, website.AccountID)
	}
	if replay := m.jobReplayLocked(job); replay != nil {
		return replay, nil
	}
	normalizeJob(job)
	if err := m.validateNewJobLocked(job); err != nil {
		return nil, err
	}
	normalizeAudit(&audit, job)
	websiteCopy := *website
	jobCopy := *job
	auditCopy := audit
	m.Websites[website.ID] = &websiteCopy
	m.Jobs[job.ID] = &jobCopy
	m.Audit = append(m.Audit, &auditCopy)
	return &jobCopy, nil
}

func (m *Memory) UpsertCertificateWithJob(certificate *Certificate, job *Job, audit AuditEvent) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if certificate == nil || job == nil {
		return nil, fmt.Errorf("certificate and job are required")
	}
	if certificate.AccountID == "" || m.Accounts[certificate.AccountID] == nil {
		return nil, fmt.Errorf("certificate account %q not found", certificate.AccountID)
	}
	if replay := m.jobReplayLocked(job); replay != nil {
		return replay, nil
	}
	for existingID, existing := range m.Certs {
		if existing.AccountID == certificate.AccountID && existing.Hostname == certificate.Hostname &&
			existingID != certificate.ID {
			return nil, fmt.Errorf("certificate already exists for %q", certificate.Hostname)
		}
	}
	normalizeJob(job)
	if err := m.validateNewJobLocked(job); err != nil {
		return nil, err
	}
	normalizeAudit(&audit, job)
	certificateCopy := *certificate
	jobCopy := *job
	auditCopy := audit
	m.Certs[certificate.ID] = &certificateCopy
	m.Jobs[job.ID] = &jobCopy
	m.Audit = append(m.Audit, &auditCopy)
	return &jobCopy, nil
}

func (m *Memory) CreateBackupWithJob(backup *BackupRun, job *Job, audit AuditEvent) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if backup == nil || job == nil {
		return nil, fmt.Errorf("backup and job are required")
	}
	if m.Accounts[backup.AccountID] == nil {
		return nil, fmt.Errorf("backup account %q not found", backup.AccountID)
	}
	if replay := m.jobReplayLocked(job); replay != nil {
		return replay, nil
	}
	if m.Backups[backup.ID] != nil {
		return nil, fmt.Errorf("backup %q already exists", backup.ID)
	}
	normalizeJob(job)
	if err := m.validateNewJobLocked(job); err != nil {
		return nil, err
	}
	normalizeAudit(&audit, job)
	backupCopy := *backup
	jobCopy := *job
	auditCopy := audit
	m.Backups[backup.ID] = &backupCopy
	m.Jobs[job.ID] = &jobCopy
	m.Audit = append(m.Audit, &auditCopy)
	return &jobCopy, nil
}

func (m *Memory) jobReplayLocked(job *Job) *Job {
	if job.IdempotencyKey == "" {
		return nil
	}
	for _, existing := range m.Jobs {
		if existing.IdempotencyKey == job.IdempotencyKey {
			copy := *existing
			return &copy
		}
	}
	return nil
}

func (m *Memory) EnqueueJobWithAudit(job *Job, audit AuditEvent) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if job == nil {
		return nil, fmt.Errorf("job is required")
	}
	if replay := m.jobReplayLocked(job); replay != nil {
		return replay, nil
	}
	normalizeJob(job)
	if err := m.validateNewJobLocked(job); err != nil {
		return nil, err
	}
	normalizeAudit(&audit, job)
	jobCopy := *job
	auditCopy := audit
	m.Jobs[job.ID] = &jobCopy
	m.Audit = append(m.Audit, &auditCopy)
	return &jobCopy, nil
}

type resourceMutation func(context.Context, pgx.Tx) error

func (p *PG) commitResourceMutation(job *Job, audit AuditEvent, mutate resourceMutation) (*Job, error) {
	if job == nil {
		return nil, fmt.Errorf("job is required")
	}
	normalizeJob(job)
	normalizeAudit(&audit, job)
	payload, err := json.Marshal(job.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal mutation job payload: %w", err)
	}
	ctx := p.ctx()
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin resource mutation: %w", err)
	}
	defer tx.Rollback(ctx)
	if job.IdempotencyKey != "" {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, job.IdempotencyKey); err != nil {
			return nil, fmt.Errorf("lock resource mutation idempotency key: %w", err)
		}
		existing, scanErr := scanJobRow(tx.QueryRow(ctx, jobSelect+" WHERE idempotency_key=$1 FOR UPDATE", job.IdempotencyKey))
		if scanErr == nil {
			return existing, nil
		}
		if !errors.Is(scanErr, pgx.ErrNoRows) {
			return nil, fmt.Errorf("lookup resource mutation replay: %w", scanErr)
		}
	}
	if err := mutate(ctx, tx); err != nil {
		return nil, err
	}
	if err := insertJobTx(ctx, tx, job, payload); err != nil {
		return nil, fmt.Errorf("insert resource mutation job: %w", err)
	}
	if err := insertAuditTx(ctx, tx, audit); err != nil {
		return nil, fmt.Errorf("insert resource mutation audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit resource mutation: %w", err)
	}
	copy := *job
	return &copy, nil
}

func insertAuditTx(ctx context.Context, tx pgx.Tx, event AuditEvent) error {
	before, err := json.Marshal(event.Before)
	if err != nil {
		return err
	}
	after, err := json.Marshal(event.After)
	if err != nil {
		return err
	}
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_events (id, occurred_at, actor_type, actor_id, effective_actor_id, account_id, action, resource_type, resource_id, source_ip, user_agent, request_id, success, before_state, after_state, metadata)
		VALUES ($1,$2,$3,NULLIF($4,'')::uuid,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,$7,$8,NULLIF($9,'')::uuid,NULLIF($10,'')::inet,$11,$12,$13,$14,$15,$16)`,
		event.ID, event.OccurredAt, event.ActorType, event.ActorID, event.EffectiveActor,
		event.AccountID, event.Action, event.ResourceType, event.ResourceID, event.SourceIP,
		event.UserAgent, event.RequestID, event.Success, before, after, metadata)
	return err
}

func (p *PG) EnqueueJobWithAudit(job *Job, audit AuditEvent) (*Job, error) {
	return p.commitResourceMutation(job, audit, func(context.Context, pgx.Tx) error {
		return nil
	})
}

func (p *PG) CreateDomainWithJob(domain *Domain, job *Job, audit AuditEvent) (*Job, error) {
	if domain == nil {
		return nil, fmt.Errorf("domain is required")
	}
	return p.commitResourceMutation(job, audit, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO domains (id, account_id, fqdn, ascii_fqdn, type, document_root, dns_managed, status, desired_revision, observed_revision)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			domain.ID, domain.AccountID, domain.FQDN, domain.ASCII, domain.Type,
			domain.DocumentRoot, domain.DNSManaged, domain.Status,
			domain.DesiredRevision, domain.ObservedRevision)
		return err
	})
}

func (p *PG) UpdateDomainWithJob(domain *Domain, job *Job, audit AuditEvent) (*Job, error) {
	if domain == nil {
		return nil, fmt.Errorf("domain is required")
	}
	return p.commitResourceMutation(job, audit, func(ctx context.Context, tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE domains SET fqdn=$3, ascii_fqdn=$4, type=$5, document_root=$6,
				dns_managed=$7, status=$8, desired_revision=$9, observed_revision=$10
			WHERE id=$1 AND account_id=$2`,
			domain.ID, domain.AccountID, domain.FQDN, domain.ASCII, domain.Type,
			domain.DocumentRoot, domain.DNSManaged, domain.Status,
			domain.DesiredRevision, domain.ObservedRevision)
		if err == nil && result.RowsAffected() != 1 {
			return fmt.Errorf("domain %q not found", domain.ID)
		}
		return err
	})
}

func (p *PG) UpsertWebsiteWithJob(website *Website, job *Job, audit AuditEvent) (*Job, error) {
	if website == nil {
		return nil, fmt.Errorf("website is required")
	}
	return p.commitResourceMutation(job, audit, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO websites (id, account_id, domain_id, runtime, runtime_version, document_root, https_redirect, www_redirect, proxy_target, enabled, desired_revision, observed_revision)
			VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,COALESCE(NULLIF($8,''),'none'),NULLIF($9,''),$10,$11,$12)
			ON CONFLICT (id) DO UPDATE SET
				runtime=EXCLUDED.runtime, runtime_version=EXCLUDED.runtime_version,
				document_root=EXCLUDED.document_root, https_redirect=EXCLUDED.https_redirect,
				www_redirect=EXCLUDED.www_redirect, proxy_target=EXCLUDED.proxy_target,
				enabled=EXCLUDED.enabled, desired_revision=EXCLUDED.desired_revision`,
			website.ID, website.AccountID, website.DomainID, website.Runtime,
			website.RuntimeVersion, website.DocumentRoot, website.HTTPSRedirect,
			website.WWWRedirect, website.ProxyTarget, website.Enabled,
			website.DesiredRevision, website.ObservedRevision)
		return err
	})
}

func (p *PG) UpsertCertificateWithJob(certificate *Certificate, job *Job, audit AuditEvent) (*Job, error) {
	if certificate == nil {
		return nil, fmt.Errorf("certificate is required")
	}
	return p.commitResourceMutation(job, audit, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO certificates (id, account_id, hostname, kind, status, not_after, issuer)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (id) DO UPDATE SET status=EXCLUDED.status,
				not_after=EXCLUDED.not_after, issuer=EXCLUDED.issuer`,
			certificate.ID, certificate.AccountID, certificate.Hostname, certificate.Kind,
			certificate.Status, certificate.NotAfter, certificate.Issuer)
		return err
	})
}

func (p *PG) CreateBackupWithJob(backup *BackupRun, job *Job, audit AuditEvent) (*Job, error) {
	if backup == nil {
		return nil, fmt.Errorf("backup is required")
	}
	return p.commitResourceMutation(job, audit, func(ctx context.Context, tx pgx.Tx) error {
		manifest, err := json.Marshal(backup.Manifest)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO backup_runs (id, account_id, kind, state, destination, checksum, size_bytes, created_at, finished_at, manifest)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			backup.ID, backup.AccountID, backup.Kind, backup.State, backup.Destination,
			backup.Checksum, backup.SizeBytes, backup.CreatedAt, backup.FinishedAt, manifest)
		return err
	})
}
