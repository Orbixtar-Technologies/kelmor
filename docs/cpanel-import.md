# cPanel import

`POST /api/v1/accounts/import/cpanel` reads an **extracted** cpmove tree (`userdata/`, `dnszones/`, `mysql.sql`, `va/`) and emits the same native export the first-party importer consumes. The control plane does not run cPanel binaries or copy trademarks.

```bash
# after extracting cpmove-acme42.tar.gz
curl -H "Authorization: Bearer $PANEL_TOKEN" -d '{"root":"/var/tmp/cpmove-acme42","username":"acme42"}' \
  http://127.0.0.1:18080/api/v1/accounts/import/cpanel
```

Homedir files are not streamed through the API (8 MiB JSON limit). Copy `homedir/` onto `/home/<user>` with the agent after the reconcile job, or restore an HPM1 backup taken from that tree.
