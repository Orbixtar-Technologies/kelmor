CREATE TABLE jobs (
    id UUID PRIMARY KEY,
    type VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50),
    resource_id UUID,
    payload JSONB NOT NULL,
    state VARCHAR(20) NOT NULL
        CHECK (state IN ('queued', 'running', 'retrying', 'succeeded', 'failed', 'cancelled')),
    priority SMALLINT NOT NULL DEFAULT 100,
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 5,
    progress INTEGER NOT NULL DEFAULT 0,
    run_after TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_by VARCHAR(255),
    locked_at TIMESTAMPTZ,
    heartbeat_at TIMESTAMPTZ,
    idempotency_key VARCHAR(255),
    last_error JSONB,
    actor_id UUID,
    request_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX jobs_idempotency
ON jobs(idempotency_key)
WHERE idempotency_key IS NOT NULL;

CREATE INDEX jobs_queue_idx
ON jobs(state, priority, run_after);

CREATE TABLE job_events (
    id UUID PRIMARY KEY,
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    message TEXT NOT NULL,
    data JSONB
);
