# Security architecture

Privilege zones:

1. Unprivileged control plane (`panel-api`, `panel-worker`, portals)
2. Root-owned typed agent (`panel-agent`) on `/run/panel/agent.sock`
3. Customer workloads as dedicated Linux users

The agent rejects paths outside approved prefixes, reserved usernames, and unknown operations. There is no `exec(string)` RPC.

Passwords use Argon2id. Secrets at rest use AES-256-GCM with `/etc/panel/secrets/master.key`. Audit events never store passwords, keys or tokens.
