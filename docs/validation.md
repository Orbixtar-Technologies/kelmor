# Validation credentials (VM, domain, SMTP relay)

Lab and public-host acceptance reads **`.run/validation/`** (gitignored). Copy the examples from `docs/validation/` if you are setting up a new machine:

```bash
mkdir -p .run/validation
cp docs/validation/domain.env.example .run/validation/domain.env
cp docs/validation/smtp.env.example .run/validation/smtp.env
cp docs/validation/vm.env.example .run/validation/vm.env
printf '%s\n' 'your-relay-password' > .run/validation/smtp_password
chmod 600 .run/validation/smtp_password .run/validation/vm.env
# optional SSH key for the public VM
# cp /path/to/key .run/validation/server-ssh-key_rsa.prv && chmod 600 .run/validation/server-ssh-key_rsa.prv
```

This tree’s local files (not committed) are:

| File | Role |
| --- | --- |
| `domain.env` | Lab zone `lab.kelmor.host`, `ns1`/`ns2.kelmor.host`, public IPv4 for PowerDNS A records |
| `smtp.env` + `smtp_password` | Authenticated SMTP2Go relay on submission port 587 (STARTTLS) |
| `vm.env` | Public Ubuntu test host (`VM_HOST` / `VM_USER` / key path) |

Paths inside those env files are **relative to `.run/validation`**, not a Windows `D:\` mount.

## Load in a shell

```bash
source scripts/load-validation-env.sh
# exports PANEL_PUBLIC_IPV4, PANEL_VALIDATION_DIR, SMTP_*, VM_*, TEST_DOMAIN, …
```

`panel-install` calls the same loader. If `--hostname` is empty it uses `TEST_DOMAIN`. `writePublicEnv` publishes `VM_PUBLIC_IPV4` as `PANEL_PUBLIC_IPV4` plus `PANEL_NS1_HOSTNAME` / `PANEL_NS2_HOSTNAME`. The mail phase writes Postfix `relayhost` and `/etc/postfix/sasl_passwd` when `SMTP_HOST` is set.

On a live host, copy the directory to `/var/lib/panel/validation` (mode `0750`, password `0600`) so resume and systemd units keep the same values:

```bash
source scripts/load-validation-env.sh
sudo scripts/sync-validation-to-host.sh
```

## SSH to the test VM

Place `server-ssh-key_rsa.prv` in `.run/validation`, then:

```bash
scripts/remote-vm.sh true
scripts/remote-vm.sh 'systemctl is-active panel-api'
```

Do not commit `smtp_password`, `vm.env`, `domain.env`, or the SSH key. Rotate the relay password if it ever lands in a public clone.
