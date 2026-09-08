# Mail

Virtual mailboxes are desired state in PostgreSQL. The worker renders Postfix `virtual`/`vdomains` maps and a Dovecot passwd-file under `/var/lib/panel/mail`. The privileged agent writes those files and, when running as root, runs `postmap` and reloads Postfix and Dovecot. Mailbox homes are `/var/vmail/<domain>/<local>/Maildir`.

The installer writes `etc/postfix/main.cf` and `etc/dovecot/dovecot.conf` for a virtual-only host (no local Unix delivery). Port 25 may be blocked on cloud networks; configure an authenticated relay if the provider requires it.

IMAP is Dovecot on 993. Submission is 587. Package `email_daily_limit` is enforced by `panel-smtp-policy` on `127.0.0.1:10031` (Postfix `check_policy_service`). Unknown senders are `DUNNO`; hosted mailbox senders increment `/var/lib/panel/mail/send-counts/<date>/<account>` and are rejected at the cap. If the policy process is down, Postfix uses `smtpd_policy_service_default_action = DUNNO`. There is no arbitrary `exec` of mail commands from the API.
