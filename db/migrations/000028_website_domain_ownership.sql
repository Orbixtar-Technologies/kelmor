LOCK TABLE domains IN SHARE ROW EXCLUSIVE MODE;
LOCK TABLE websites IN SHARE ROW EXCLUSIVE MODE;

-- A mismatched website cannot safely remain active. The domain is authoritative
-- for ownership, so quarantine the website while moving it to that account.
UPDATE websites AS website
SET account_id = domain.account_id,
    enabled = FALSE,
    desired_revision = GREATEST(website.desired_revision, website.observed_revision) + 1
FROM domains AS domain
WHERE domain.id = website.domain_id
  AND website.account_id <> domain.account_id;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'domains'::regclass
          AND conname = 'domains_account_id_id_key'
    ) THEN
        ALTER TABLE domains
            ADD CONSTRAINT domains_account_id_id_key
            UNIQUE (account_id, id);
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'websites'::regclass
          AND conname = 'websites_account_domain_fkey'
    ) THEN
        ALTER TABLE websites
            ADD CONSTRAINT websites_account_domain_fkey
            FOREIGN KEY (account_id, domain_id)
            REFERENCES domains(account_id, id)
            ON DELETE RESTRICT
            NOT VALID;
    END IF;
END
$$;

ALTER TABLE websites
    VALIDATE CONSTRAINT websites_account_domain_fkey;
