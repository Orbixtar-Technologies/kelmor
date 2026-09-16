ALTER TABLE resource_usage
    ADD COLUMN IF NOT EXISTS disk_limited BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS bandwidth_hold BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS enforced_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS certificates_renewal_idx
    ON certificates (not_after ASC)
    WHERE status = 'active' AND not_after IS NOT NULL;

CREATE INDEX IF NOT EXISTS audit_events_account_time_idx
    ON audit_events (account_id, occurred_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS audit_events_action_time_idx
    ON audit_events (action, occurred_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS dns_zones_name_idx ON dns_zones (name);

CREATE INDEX IF NOT EXISTS backup_runs_account_created_idx
    ON backup_runs (account_id, created_at DESC);
