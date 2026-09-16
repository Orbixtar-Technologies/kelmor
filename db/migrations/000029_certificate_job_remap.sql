LOCK TABLE certificates IN SHARE ROW EXCLUSIVE MODE;
LOCK TABLE jobs IN SHARE ROW EXCLUSIVE MODE;

DROP INDEX IF EXISTS certificates_account_hostname_uidx;

-- Older jobs that already lost their certificate in migration 000023 can only
-- be repaired when account and hostname metadata identify one exact survivor.
UPDATE jobs AS job
SET resource_id = certificate.id,
    payload = jsonb_set(
        job.payload,
        '{certificate_id}',
        to_jsonb(certificate.id::TEXT)
    )
FROM certificates AS certificate
WHERE job.type = 'certificate.provision'
  AND NOT EXISTS (
      SELECT 1 FROM certificates AS referenced
      WHERE referenced.id = job.resource_id
  )
  AND certificate.account_id::TEXT = job.payload->>'account_id'
  AND certificate.hostname = job.payload->>'hostname';

CREATE TEMPORARY TABLE certificate_dedupe_map (
    duplicate_id UUID PRIMARY KEY,
    survivor_id UUID NOT NULL
) ON COMMIT DROP;

INSERT INTO certificate_dedupe_map (duplicate_id, survivor_id)
SELECT id, survivor_id
FROM (
    SELECT
        id,
        first_value(id) OVER (
            PARTITION BY account_id, hostname
            ORDER BY
                CASE status
                    WHEN 'active' THEN 0
                    WHEN 'renewing' THEN 1
                    WHEN 'requested' THEN 2
                    ELSE 3
                END,
                not_after DESC NULLS LAST,
                created_at DESC,
                id
        ) AS survivor_id,
        row_number() OVER (
            PARTITION BY account_id, hostname
            ORDER BY
                CASE status
                    WHEN 'active' THEN 0
                    WHEN 'renewing' THEN 1
                    WHEN 'requested' THEN 2
                    ELSE 3
                END,
                not_after DESC NULLS LAST,
                created_at DESC,
                id
        ) AS duplicate_rank
    FROM certificates
) AS ranked
WHERE duplicate_rank > 1;

UPDATE jobs AS job
SET resource_id = mapping.survivor_id,
    payload = CASE
        WHEN job.payload->>'certificate_id' = mapping.duplicate_id::TEXT
            THEN jsonb_set(
                job.payload,
                '{certificate_id}',
                to_jsonb(mapping.survivor_id::TEXT)
            )
        ELSE job.payload
    END
FROM certificate_dedupe_map AS mapping
WHERE job.type = 'certificate.provision'
  AND (
      job.resource_id = mapping.duplicate_id
      OR job.payload->>'certificate_id' = mapping.duplicate_id::TEXT
  );

DELETE FROM certificates AS certificate
USING certificate_dedupe_map AS mapping
WHERE certificate.id = mapping.duplicate_id;

-- Retain ambiguous jobs as operation history, but never dispatch them against
-- an arbitrary tenant certificate.
UPDATE jobs AS job
SET state = CASE
        WHEN job.state IN ('queued', 'running', 'retrying') THEN 'failed'
        ELSE job.state
    END,
    last_error = CASE
        WHEN job.state IN ('queued', 'running', 'retrying')
            THEN jsonb_build_object('error', 'orphaned_certificate_reference')
        ELSE job.last_error
    END,
    locked_by = CASE
        WHEN job.state IN ('queued', 'running', 'retrying') THEN NULL
        ELSE job.locked_by
    END,
    locked_at = CASE
        WHEN job.state IN ('queued', 'running', 'retrying') THEN NULL
        ELSE job.locked_at
    END,
    heartbeat_at = CASE
        WHEN job.state IN ('queued', 'running', 'retrying') THEN NULL
        ELSE job.heartbeat_at
    END,
    finished_at = CASE
        WHEN job.state IN ('queued', 'running', 'retrying') THEN now()
        ELSE job.finished_at
    END
WHERE job.type = 'certificate.provision'
  AND NOT EXISTS (
      SELECT 1 FROM certificates AS certificate
      WHERE certificate.id = job.resource_id
  );

CREATE UNIQUE INDEX certificates_account_hostname_uidx
    ON certificates (account_id, hostname);
