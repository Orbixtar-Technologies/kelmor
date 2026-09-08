CREATE TABLE backup_policies (
    id UUID PRIMARY KEY,
    account_id UUID REFERENCES accounts(id),
    name VARCHAR(80) NOT NULL,
    schedule TEXT NOT NULL,
    retain_days INTEGER NOT NULL,
    destination VARCHAR(20) NOT NULL CHECK (destination IN ('local', 'sftp', 's3')),
    destination_config_enc BYTEA,
    encrypt BOOLEAN NOT NULL DEFAULT TRUE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE backup_runs (
    id UUID PRIMARY KEY,
    policy_id UUID REFERENCES backup_policies(id),
    account_id UUID NOT NULL REFERENCES accounts(id),
    kind VARCHAR(20) NOT NULL,
    state VARCHAR(20) NOT NULL,
    destination VARCHAR(20) NOT NULL,
    checksum TEXT,
    size_bytes BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ
);

CREATE INDEX backup_runs_account_idx ON backup_runs(account_id, created_at DESC);
