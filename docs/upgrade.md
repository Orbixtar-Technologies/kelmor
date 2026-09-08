# Upgrade

`panel-updater verify` checks an Ed25519-signed release manifest. Payloads that say `unsigned-development` are refused. After verify, the operator (or a signed installer phase) locks the host, copies hashed artifacts, dumps the control database, switches `/usr/local/panel/current`, and restarts `panel-api`, `panel-worker`, and `panel-agent`.

The updater never runs `apt-get dist-upgrade` or other unattended OS upgrades.
