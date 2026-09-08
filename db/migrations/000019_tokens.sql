CREATE TABLE api_tokens (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(80) NOT NULL,
    prefix VARCHAR(24) NOT NULL UNIQUE,
    token_hash BYTEA NOT NULL UNIQUE,
    scope VARCHAR(20) NOT NULL CHECK (scope IN ('server', 'reseller', 'account')),
    account_id UUID REFERENCES accounts(id),
    reseller_id UUID REFERENCES resellers(id),
    capabilities TEXT[] NOT NULL,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
