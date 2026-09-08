# Mail

Virtual mailboxes are desired state in PostgreSQL. The worker renders Postfix `virtual`/`vdomains` maps and a Dovecot passwd-file under `/var/lib/panel/mail`. The privileged agent writes those files and, when running as root, runs `postmap` and reloads Postfix and Dovecot. Mailbox homes are `/var/vmail/<domain>/<local>/Maildir`.

The installer writes `etc/postfix/main.cf` and `etc/dovecot/dovecot.conf` for a virtual-only host (no local Unix delivery). Port 25 may be blocked on cloud networks; configure an authenticated relay if the provider requires it.

IMAP is Dovecot on 993. Submission is 587. There is no arbitrary `exec` of mail commands from the API.
