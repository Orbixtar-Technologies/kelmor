CREATE TABLE accounts (
    id UUID PRIMARY KEY,
    reseller_id UUID REFERENCES resellers(id),
    owner_user_id UUID NOT NULL REFERENCES users(id),
    username VARCHAR(32) NOT NULL UNIQUE,
    primary_domain VARCHAR(253) NOT NULL,
    linux_uid INTEGER NOT NULL UNIQUE,
    linux_gid INTEGER NOT NULL,
    package_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL
        CHECK (status IN (
            'provisioning',
            'active',
            'suspended',
            'terminating',
            'terminated',
            'failed'
        )),
    home_path TEXT NOT NULL,
    ip_address INET,
    shell_class VARCHAR(32) NOT NULL DEFAULT 'sftp-only',
    login_disabled BOOLEAN NOT NULL DEFAULT FALSE,
    desired_revision BIGINT NOT NULL DEFAULT 1,
    observed_revision BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE account_members (
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(40) NOT NULL,
    PRIMARY KEY (account_id, user_id)
);

CREATE INDEX accounts_reseller_status_idx ON accounts(reseller_id, status);
CREATE INDEX accounts_username_idx ON accounts(username);
