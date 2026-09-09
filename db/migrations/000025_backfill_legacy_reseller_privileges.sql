-- Empty reseller masks previously inherited this default capability set; preserve that behavior for existing rows only.
UPDATE resellers
SET privilege_mask = ARRAY[
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
]::TEXT[]
WHERE cardinality(privilege_mask) = 0;
