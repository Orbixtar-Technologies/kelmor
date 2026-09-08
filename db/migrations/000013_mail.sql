CREATE TABLE mail_domains (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES accounts(id),
    domain_id UUID NOT NULL REFERENCES domains(id),
    catchall_policy VARCHAR(20) NOT NULL DEFAULT 'reject',
    daily_send_limit INTEGER,
    status VARCHAR(20) NOT NULL DEFAULT 'provisioning'
);

CREATE TABLE mailboxes (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES accounts(id),
    domain_id UUID NOT NULL REFERENCES mail_domains(id),
    local_part VARCHAR(64) NOT NULL,
    quota_bytes BIGINT NOT NULL,
    password_hash TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    UNIQUE (domain_id, local_part)
);

CREATE TABLE mail_aliases (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES accounts(id),
    domain_id UUID NOT NULL REFERENCES mail_domains(id),
    address VARCHAR(320) NOT NULL,
    destination TEXT NOT NULL
);

CREATE INDEX mailboxes_account_idx ON mailboxes(account_id);
