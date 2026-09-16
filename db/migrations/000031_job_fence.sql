ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS fence BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS operation_id TEXT,
    ADD COLUMN IF NOT EXISTS lease_expires TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS cancel_requested BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE jobs
    DROP CONSTRAINT IF EXISTS jobs_fence_nonnegative;

ALTER TABLE jobs
    ADD CONSTRAINT jobs_fence_nonnegative
    CHECK (fence >= 0)
    NOT VALID;

ALTER TABLE jobs
    VALIDATE CONSTRAINT jobs_fence_nonnegative;

CREATE TABLE IF NOT EXISTS resource_fences (
    resource_key TEXT PRIMARY KEY,
    fence BIGINT NOT NULL DEFAULT 0
        CHECK (fence >= 0)
);
