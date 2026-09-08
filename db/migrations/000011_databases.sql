CREATE TABLE database_servers (
    id UUID PRIMARY KEY,
    engine VARCHAR(20) NOT NULL CHECK (engine IN ('mariadb', 'postgres')),
    name VARCHAR(80) NOT NULL UNIQUE,
    host TEXT NOT NULL,
    port INTEGER NOT NULL
);

CREATE TABLE hosted_databases (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES accounts(id),
    server_id UUID NOT NULL REFERENCES database_servers(id),
    name VARCHAR(64) NOT NULL,
    engine VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'provisioning',
    UNIQUE (server_id, name)
);

CREATE TABLE database_users (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES accounts(id),
    username VARCHAR(64) NOT NULL UNIQUE,
    engine VARCHAR(20) NOT NULL,
    password_enc BYTEA NOT NULL
);

CREATE TABLE database_grants (
    id UUID PRIMARY KEY,
    database_id UUID NOT NULL REFERENCES hosted_databases(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES database_users(id) ON DELETE CASCADE,
    privileges TEXT[] NOT NULL
);
