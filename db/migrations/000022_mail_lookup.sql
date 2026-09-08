DO $$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'panel_mail_lookup') THEN
    CREATE ROLE panel_mail_lookup NOLOGIN;
  END IF;
END $$;

CREATE OR REPLACE VIEW mail_lookup_domains AS
SELECT md.id, d.ascii_fqdn AS domain, md.catchall_policy, md.status
FROM mail_domains md
JOIN domains d ON d.id = md.domain_id;

CREATE OR REPLACE VIEW mail_lookup_mailboxes AS
SELECT m.id, d.ascii_fqdn AS domain, m.local_part, m.quota_bytes, m.status
FROM mailboxes m
JOIN mail_domains md ON md.id = m.domain_id
JOIN domains d ON d.id = md.domain_id;

GRANT SELECT ON mail_lookup_domains, mail_lookup_mailboxes TO panel_mail_lookup;
