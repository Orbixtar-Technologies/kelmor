# cPanel import

`POST /api/v1/accounts/import/cpanel` reads an **extracted** cpmove tree (`userdata/`, `dnszones/`, `mysql.sql`, `va/`) and emits the same native export the first-party importer consumes. The control plane does not run cPanel binaries or copy trademarks.

```bash
# after extracting cpmove-acme42.tar.gz
curl -H "Authorization: Bearer $PANEL_TOKEN" -d '{"root":"/var/tmp/cpmove-acme42","username":"acme42"}' \
  http://127.0.0.1:18080/api/v1/accounts/import/cpanel
```

The importer enqueues `account.reconcile` plus `account.copy_homedir`. The worker calls the typed agent `CopyHomedir` operation, which copies `homedir/` (regular files only, no symlinks) onto `/home/<username>`. Extract cpmove trees under `/var/lib/panel/imports/` or `/var/tmp/panel-imports/` so the agent path policy accepts the source. The 8 MiB JSON API limit is not used for file bytes.
