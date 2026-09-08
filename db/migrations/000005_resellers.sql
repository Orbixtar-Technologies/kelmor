CREATE TABLE resellers (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    name VARCHAR(120) NOT NULL,
    brand_name TEXT,
    brand_color TEXT,
    nameservers TEXT[] NOT NULL DEFAULT '{}',
    privilege_mask TEXT[] NOT NULL DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
