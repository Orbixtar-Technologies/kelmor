CREATE TABLE audit_events (
    id UUID PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor_type VARCHAR(30) NOT NULL,
    actor_id UUID,
    effective_actor_id UUID,
    account_id UUID,
    action VARCHAR(120) NOT NULL,
    resource_type VARCHAR(80),
    resource_id UUID,
    source_ip INET,
    user_agent TEXT,
    request_id UUID NOT NULL,
    success BOOLEAN NOT NULL,
    before_state JSONB,
    after_state JSONB,
    metadata JSONB
);

CREATE INDEX audit_events_time_idx ON audit_events(occurred_at DESC);
CREATE INDEX audit_events_actor_idx ON audit_events(actor_id, occurred_at DESC);
CREATE INDEX audit_events_resource_idx ON audit_events(resource_id, occurred_at DESC);
