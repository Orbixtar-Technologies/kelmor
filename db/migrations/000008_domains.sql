CREATE TABLE domains (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES accounts(id),
    fqdn VARCHAR(253) NOT NULL,
    ascii_fqdn VARCHAR(253) NOT NULL UNIQUE,
    type VARCHAR(20) NOT NULL
        CHECK (type IN ('primary', 'addon', 'alias', 'subdomain')),
    document_root TEXT,
    dns_managed BOOLEAN NOT NULL DEFAULT TRUE,
    status VARCHAR(20) NOT NULL DEFAULT 'provisioning',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX domains_account_idx ON domains(account_id);
