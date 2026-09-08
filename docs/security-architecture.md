# Security architecture

Privilege zones:

1. Unprivileged control plane (`panel-api`, `panel-worker`, portals)
2. Root-owned typed agent (`panel-agent`) on `/run/panel/agent.sock`
3. Customer workloads as dedicated Linux users

The agent rejects paths outside approved prefixes, reserved usernames, and unknown operations. There is no `exec(string)` RPC.

Inbound traffic is enforced by nftables `table inet panel` (drop policy, loopback and established allowed, hosting ports including FTP 21/PASV 40000-40100 plus already-bound management listeners). Tenant SFTP is chrooted to `/home/<user>` via `Match Group panel-sftp`. Virtual FTP users live in `/var/lib/panel/ftp/passwd` (SHA-512 crypt) with per-user `guest_username` maps; vsftpd never takes a shell command from the API. Package `disk_bytes` is written to `/var/lib/panel/quotas/<user>` and `/home/<user>/.panel-quota`. When usage meets the cap, the agent makes `public_html` (and sibling write trees) mode `0550` and sets `ForceCommand internal-sftp -R` for that user so SFTP cannot bypass the API disk check. Kernel `setquota` is still applied when `CONFIG_QUOTA` exists.

Passwords use Argon2id. Secrets at rest use AES-256-GCM with `/etc/panel/secrets/master.key`. Audit events never store passwords, keys or tokens.
