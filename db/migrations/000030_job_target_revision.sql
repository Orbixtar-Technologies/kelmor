ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS target_revision BIGINT NOT NULL DEFAULT 0;

ALTER TABLE jobs
    DROP CONSTRAINT IF EXISTS jobs_target_revision_nonnegative;

ALTER TABLE jobs
    ADD CONSTRAINT jobs_target_revision_nonnegative
    CHECK (target_revision >= 0)
    NOT VALID;

ALTER TABLE jobs
    VALIDATE CONSTRAINT jobs_target_revision_nonnegative;

ALTER TABLE domains
    ADD COLUMN IF NOT EXISTS desired_revision BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS observed_revision BIGINT NOT NULL DEFAULT 0;
