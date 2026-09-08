CREATE TABLE dns_zones (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES accounts(id),
    domain_id UUID NOT NULL REFERENCES domains(id),
    name VARCHAR(253) NOT NULL UNIQUE,
    dnssec_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    provider VARCHAR(40) NOT NULL DEFAULT 'powerdns',
    desired_revision BIGINT NOT NULL DEFAULT 1,
    observed_revision BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE dns_records (
    id UUID PRIMARY KEY,
    zone_id UUID NOT NULL REFERENCES dns_zones(id) ON DELETE CASCADE,
    name VARCHAR(253) NOT NULL,
    type VARCHAR(10) NOT NULL,
    content TEXT NOT NULL,
    ttl INTEGER NOT NULL DEFAULT 3600,
    priority INTEGER
);

CREATE INDEX dns_records_zone_idx ON dns_records(zone_id);
