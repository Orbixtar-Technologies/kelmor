CREATE TABLE servers (
    id UUID PRIMARY KEY,
    hostname VARCHAR(253) NOT NULL UNIQUE,
    os VARCHAR(64) NOT NULL,
    kernel TEXT,
    public_ip INET,
    status VARCHAR(20) NOT NULL DEFAULT 'online',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE server_services (
    id UUID PRIMARY KEY,
    server_id UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    name VARCHAR(64) NOT NULL,
    desired_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    observed_running BOOLEAN NOT NULL DEFAULT FALSE,
    health VARCHAR(20) NOT NULL DEFAULT 'unknown'
        CHECK (health IN ('healthy', 'degraded', 'failed', 'unknown', 'maintenance')),
    last_checked_at TIMESTAMPTZ,
    last_failure TEXT,
    UNIQUE (server_id, name)
);

CREATE TABLE server_ips (
    id UUID PRIMARY KEY,
    server_id UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    address INET NOT NULL UNIQUE,
    family SMALLINT NOT NULL,
    assigned_account_id UUID,
    reserved BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE id_allocators (
    name TEXT PRIMARY KEY,
    next_value BIGINT NOT NULL
);

INSERT INTO id_allocators (name, next_value) VALUES ('linux_uid', 20000);
