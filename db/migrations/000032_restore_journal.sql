CREATE TABLE IF NOT EXISTS restore_journals (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    backup_id TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    object_key TEXT NOT NULL,
    manifest_hash TEXT,
    format_version INTEGER NOT NULL DEFAULT 3,
    key_identity TEXT,
    key_version INTEGER NOT NULL DEFAULT 1,
    fence BIGINT NOT NULL DEFAULT 0,
    state TEXT NOT NULL,
    manual_intervention BOOLEAN NOT NULL DEFAULT FALSE,
    checkpoints JSONB NOT NULL DEFAULT '[]'::jsonb,
    extra JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS restore_journals_one_active
    ON restore_journals (account_id)
    WHERE state NOT IN ('complete', 'failed');
