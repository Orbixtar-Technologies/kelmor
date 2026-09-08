# Native account migration

`GET /api/v1/accounts/{id}/export` returns a versioned JSON document (`format_version: 1`) with the account, domains, websites, databases, mail domains/mailboxes, DNS, cron, and FTP rows.

`POST /api/v1/accounts/import` refuses a colliding username or primary domain, writes the rows, and enqueues `account.reconcile` so the typed agent rebuilds Linux identity, Nginx, mail maps, zone files, and certificates.

This is the first-party move path. A cPanel importer is a separate adapter that must emit this same document; it is not a second control plane.
