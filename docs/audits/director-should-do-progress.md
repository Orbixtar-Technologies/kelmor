# Director Should-do progress

Tracks `docs/audits/2026-09-19-director-nav-page-review.md` rows that this
branch implements as host-applied work (API → job → Agent) instead of
`director-settings.json` preference saves.

| Audit id | Slice | Status | Implementation |
| --- | --- | --- | --- |
| `change-site-ip` | A | Done | PATCH `ip_address` (validated) → reconcile publishes A/SPF from dedicated IPv4 |
| `assign-ipv6` | A | Done | PATCH IPv6 → reconcile publishes AAAA; IPv4 stays shared/`publicIPv4()` |
| `manage-shell` | A | Done | PATCH `shell_class` → `CreateLinuxUser` / `UnlockLinuxUser` POSIX shell |
| `unsuspend-bandwidth` | A | Done | `POST /accounts/bulk/clear-bandwidth-hold` only; not all suspended |
| `file-dir-restore` | A | Done | `POST /accounts/{id}/restores` with `path` + latest backup + unpack prefix |
| Job notices / cancel | A | Done | `QueuedOpNotice` on generic tools; Jobs cancel for queued/failed |
| Settings PATCH | B–D | Done | `PATCH /server/settings` queues `host.config.apply` for host keys |
| `email-all-users` / `email-resellers` | B | Done | `POST /mail/notify` → `mail.notify` → Agent `SendSystemMail` |
| `synchronize-dns` / `dns-cleanup` | B | Done | `POST /dns/synchronize` and `/dns/cleanup` |
| Mail filters / greylist / spamd / smtp | B | Done | `ApplyHostConfig` writes rspamd local.d + Postfix restrict |
| `change-hostname` / resolvers / time | C | Done | hostnamectl, resolvers.conf, timedatectl via Agent |
| `host-access` / `cphulk` / password / idle | C | Done | nft CIDRs; login limiter + session TTL + password policy |
| `ssh-keys-root` / `wheel-group` / compiler | C | Done | Agent writes panel files and applies on live hosts |
| `backup-config` / `backup-user-selection` / cron | D | Done | cron.d + 24h selected-account backup scan |
| `multi-modify` / `multi-ip` / ownership / `ip-migration` | D | Done | `POST /accounts/bulk/modify` and `/accounts/ip-migration` |
| `convert-addon` | D | Done | `POST /accounts/convert-addon` provisions a new account |
| `theme-manager` / `locales` / `customization` | D | Done | Director chrome (`data-density`, `lang`) — no Agent job |
| `module-installers` / `perl-modules` / `php-pear` / `php-pecl` / `ruby-gems` | Software | Done | `GET/POST /server/modules` → `host.module.install` → Agent apt/pecl/pear/cpan/gem |
| `server-profile` | C | Done | `PATCH` settings → `host.config.apply` writes `/etc/panel/server-profile` and toggles tenant units |
| `configuration-cluster` | Clusters | Done | Peer URLs on host + `POST /server/cluster/publish` snapshot. No live multi-node orchestration |
| `grant-support-access` | Support | Done | Time-limited `server_operator` session for `kelmor-support` |
| `diagnostics-log` | Support | Done | Authenticated `GET /server/diagnostics` tar.gz download |

Rows that stay honest status (cannot be a real host product yet) are noted in
the audit file under **Deferred / omitted**.
