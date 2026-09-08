CREATE TABLE resource_usage (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES accounts(id),
    collected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    disk_bytes BIGINT NOT NULL DEFAULT 0,
    inode_count BIGINT NOT NULL DEFAULT 0,
    bandwidth_bytes BIGINT NOT NULL DEFAULT 0,
    cpu_percent NUMERIC(6,2) NOT NULL DEFAULT 0,
    memory_bytes BIGINT NOT NULL DEFAULT 0,
    process_count INTEGER NOT NULL DEFAULT 0,
    mail_bytes BIGINT NOT NULL DEFAULT 0,
    database_bytes BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX resource_usage_account_idx ON resource_usage(account_id, collected_at DESC);

CREATE TABLE ftp_accounts (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES accounts(id),
    username VARCHAR(64) NOT NULL UNIQUE,
    home_path TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active'
);

CREATE TABLE ssh_keys (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES accounts(id),
    label TEXT NOT NULL,
    public_key TEXT NOT NULL,
    fingerprint TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE cron_jobs (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES accounts(id),
    schedule TEXT NOT NULL,
    command TEXT NOT NULL,
    working_directory TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE service_credentials (
    id UUID PRIMARY KEY,
    name VARCHAR(80) NOT NULL UNIQUE,
    encrypted_value BYTEA NOT NULL,
    rotated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(64) PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
