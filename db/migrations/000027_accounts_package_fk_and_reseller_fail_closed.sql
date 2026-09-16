DO $$
DECLARE
    duplicate_constraint RECORD;
BEGIN
    FOR duplicate_constraint IN
        SELECT constraint_row.conname
        FROM pg_constraint AS constraint_row
        JOIN pg_class AS account_table ON account_table.oid = constraint_row.conrelid
        JOIN pg_namespace AS account_schema ON account_schema.oid = account_table.relnamespace
        JOIN pg_class AS package_table ON package_table.oid = constraint_row.confrelid
        JOIN pg_attribute AS package_column
          ON package_column.attrelid = account_table.oid
         AND package_column.attnum = ANY (constraint_row.conkey)
        WHERE constraint_row.contype = 'f'
          AND account_schema.nspname = current_schema()
          AND account_table.relname = 'accounts'
          AND package_table.relname = 'packages'
          AND package_column.attname = 'package_id'
          AND cardinality(constraint_row.conkey) = 1
          AND constraint_row.conname <> 'accounts_package_id_fkey'
    LOOP
        EXECUTE format(
            'ALTER TABLE accounts DROP CONSTRAINT %I',
            duplicate_constraint.conname
        );
    END LOOP;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'accounts'::regclass
          AND conname = 'accounts_package_id_fkey'
    ) THEN
        ALTER TABLE accounts
            ADD CONSTRAINT accounts_package_id_fkey
            FOREIGN KEY (package_id)
            REFERENCES packages(id)
            ON DELETE RESTRICT
            NOT VALID;
    END IF;
END
$$;

ALTER TABLE accounts
    VALIDATE CONSTRAINT accounts_package_id_fkey;

-- Migration 000025 could not distinguish an intentionally empty deny-all mask
-- from an old row that inherited the historical default. Ambiguity is denied.
UPDATE resellers
SET privilege_mask = ARRAY[]::TEXT[]
WHERE privilege_mask = ARRAY[
    'accounts.read',
    'accounts.create',
    'accounts.modify',
    'accounts.suspend',
    'packages.read',
    'packages.write',
    'domains.read',
    'domains.write',
    'dns.read',
    'websites.read',
    'backups.read',
    'backups.create',
    'backups.restore',
    'billing.usage.read'
]::TEXT[];
