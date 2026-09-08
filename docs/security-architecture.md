# Security architecture

Privilege zones:

1. Unprivileged control plane (`panel-api`, `panel-worker`, portals)
2. Root-owned typed agent (`panel-agent`) on `/run/panel/agent.sock`
3. Customer workloads as dedicated Linux users

The agent rejects paths outside approved prefixes, reserved usernames, and unknown operations. There is no `exec(string)` RPC.

Inbound traffic is enforced by nftables `table inet panel` (drop policy, loopback and established allowed, hosting ports plus already-bound management listeners). Tenant SFTP is chrooted to `/home/<user>` via `Match Group panel-sftp`.

Passwords use Argon2id. Secrets at rest use AES-256-GCM with `/etc/panel/secrets/master.key`. Audit events never store passwords, keys or tokens.
