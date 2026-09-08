CREATE TABLE feature_sets (
    id UUID PRIMARY KEY,
    name VARCHAR(80) NOT NULL UNIQUE,
    features JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE packages (
    id UUID PRIMARY KEY,
    reseller_id UUID REFERENCES resellers(id),
    name VARCHAR(80) NOT NULL,
    feature_set_id UUID NOT NULL REFERENCES feature_sets(id),
    disk_bytes BIGINT NOT NULL,
    bandwidth_bytes_monthly BIGINT NOT NULL,
    domains INTEGER NOT NULL,
    subdomains INTEGER NOT NULL,
    alias_domains INTEGER NOT NULL,
    databases INTEGER NOT NULL,
    database_users INTEGER NOT NULL,
    mailboxes INTEGER NOT NULL,
    mailbox_storage_bytes BIGINT NOT NULL,
    ftp_users INTEGER NOT NULL,
    cron_jobs INTEGER NOT NULL,
    application_instances INTEGER NOT NULL,
    backup_retention_days INTEGER NOT NULL,
    cpu_percent INTEGER NOT NULL,
    memory_bytes BIGINT NOT NULL,
    process_limit INTEGER NOT NULL,
    io_weight INTEGER NOT NULL,
    iops INTEGER NOT NULL,
    concurrent_web_requests INTEGER NOT NULL,
    email_daily_limit INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (reseller_id, name)
);

ALTER TABLE accounts
    ADD CONSTRAINT accounts_package_fk
    FOREIGN KEY (package_id) REFERENCES packages(id);
