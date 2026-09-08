CREATE TABLE websites (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES accounts(id),
    domain_id UUID NOT NULL REFERENCES domains(id),
    runtime VARCHAR(20) NOT NULL
        CHECK (runtime IN ('static', 'php', 'node', 'python', 'proxy')),
    runtime_version VARCHAR(32),
    document_root TEXT NOT NULL,
    https_redirect BOOLEAN NOT NULL DEFAULT TRUE,
    www_redirect VARCHAR(20) NOT NULL DEFAULT 'none',
    proxy_target TEXT,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    desired_revision BIGINT NOT NULL DEFAULT 1,
    observed_revision BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX websites_account_idx ON websites(account_id);
