CREATE TABLE applications (
    id UUID PRIMARY KEY,
    website_id UUID NOT NULL REFERENCES websites(id),
    account_id UUID NOT NULL REFERENCES accounts(id),
    runtime VARCHAR(20) NOT NULL,
    runtime_version VARCHAR(32) NOT NULL,
    working_directory TEXT NOT NULL,
    start_command TEXT NOT NULL,
    listen_type VARCHAR(20) NOT NULL DEFAULT 'unix',
    listen_target TEXT NOT NULL,
    instances INTEGER NOT NULL DEFAULT 1,
    restart_policy VARCHAR(20) NOT NULL DEFAULT 'on-failure',
    status VARCHAR(20) NOT NULL DEFAULT 'stopped',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE application_env (
    id UUID PRIMARY KEY,
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    encrypted_value BYTEA NOT NULL,
    is_secret BOOLEAN NOT NULL DEFAULT TRUE,
    UNIQUE (application_id, name)
);

CREATE TABLE wordpress_installations (
    id UUID PRIMARY KEY,
    website_id UUID NOT NULL REFERENCES websites(id),
    account_id UUID NOT NULL REFERENCES accounts(id),
    path TEXT NOT NULL,
    version TEXT,
    url TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'detected',
    auto_update_policy VARCHAR(20) NOT NULL DEFAULT 'minor',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
