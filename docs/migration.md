# Native account migration

`GET /api/v1/accounts/{id}/export` returns a versioned JSON document (`format_version: 1`) with the account, domains, websites, applications, databases, mail domains/mailboxes/aliases (plus mailbox password hashes), DNS, cron, FTP (plus hashes), and SSH keys.

`POST /api/v1/accounts/{id}/migrate` and `POST /api/v1/accounts/import` refuse a colliding username or any remapped domain. Import remaps `/home/<user>` paths, `{user}_` database names, and every FQDN under the primary (aliases and addons included). It writes cron, FTP, aliases, applications, and SSH rows, then enqueues one `account.reconcile` job whose payload lists `copy_source`, database dump/restore pairs, and mailbox Maildir pairs.

After the Linux user exists, the worker copies the home tree, dumps each source MariaDB/PostgreSQL database into the remapped name, and unpacks each Maildir under `/var/vmail/<dest-domain>/<local>`. Certificates are re-issued for the new hostnames. The source account is left in place.

This is the first-party move path. A cPanel importer is a separate adapter that must emit this same document; it is not a second control plane. cPanel trees copy `homedir` only and create empty hosted databases (SQL dumps in the tree are parsed as names, not replayed).
