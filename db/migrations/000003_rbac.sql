CREATE TABLE roles (
    id UUID PRIMARY KEY,
    name VARCHAR(64) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE permissions (
    id UUID PRIMARY KEY,
    capability VARCHAR(120) NOT NULL UNIQUE
);

CREATE TABLE role_permissions (
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    account_id UUID,
    reseller_id UUID
);

CREATE UNIQUE INDEX user_roles_uniq ON user_roles (
    user_id,
    role_id,
    COALESCE(account_id, '00000000-0000-0000-0000-000000000000'),
    COALESCE(reseller_id, '00000000-0000-0000-0000-000000000000')
);
