CREATE TABLE certificates (
    id UUID PRIMARY KEY,
    account_id UUID REFERENCES accounts(id),
    hostname VARCHAR(253) NOT NULL,
    kind VARCHAR(20) NOT NULL CHECK (kind IN ('panel', 'mail', 'domain', 'wildcard')),
    status VARCHAR(20) NOT NULL
        CHECK (status IN ('requested', 'validating', 'issued', 'active', 'renewing', 'failed', 'expired', 'revoked')),
    not_after TIMESTAMPTZ,
    issuer TEXT,
    fingerprint TEXT,
    cert_pem TEXT,
    key_enc BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
