# Mail

Virtual mailboxes are desired state in PostgreSQL. The worker renders Postfix `virtual`/`vdomains` maps and a Dovecot passwd-file under `/var/lib/panel/mail`. The privileged agent writes those files and, when running as root, runs `postmap` and reloads Postfix and Dovecot. Mailbox homes are `/var/vmail/<domain>/<local>/Maildir`.

The installer writes `etc/postfix/main.cf` and `etc/dovecot/dovecot.conf` for a virtual-only host (no local Unix delivery). Port 25 may be blocked on cloud networks; configure an authenticated relay if the provider requires it.

IMAP is Dovecot on 993. Authenticated submission is Postfix on **587** (STARTTLS required) using Dovecot SASL against `/var/lib/panel/mail/passwd` (`info@domain` + mailbox password). Port 25 stays inbound MX only (`permit_mynetworks`, `reject_unauth_destination`) so the host is not an open relay. Package `email_daily_limit` is enforced by `panel-smtp-policy` on `127.0.0.1:10031` (Postfix `check_policy_service`). Unknown senders are `DUNNO`; hosted mailbox senders increment `/var/lib/panel/mail/send-counts/<date>/<account>` and are rejected at the cap. If the policy process is down, Postfix uses `smtpd_policy_service_default_action = DUNNO`. There is no arbitrary `exec` of mail commands from the API.

Each mail domain has a catch-all policy (`reject`, `discard`, or a mailbox local part). `reject` leaves unknown recipients unlisted so Postfix bounces them. `discard` and a local part add `@domain` to the virtual mailbox map and create `/var/vmail/<domain>/<local>/Maildir`. Change it with `PATCH /api/v1/accounts/{id}/mail/domains/{mailDomainID}` or Account Portal → Email.

Virtual aliases (`mail_aliases`) rewrite recipients through Postfix `virtual_alias_maps` (`/var/lib/panel/mail/aliases`). Destination is another local part on the same domain or a full address. Creating or deleting an alias, or deleting a mailbox, queues a maps apply. A mailbox that is still an alias destination cannot be deleted.
