# Validation credentials (local only)

This directory holds **lab** credentials for the public test VM, authoritative DNS names, and the SMTP relay. The files themselves are gitignored (see `.gitignore`). Checked-in copies without secrets live in `docs/validation/`.

| File | Used for |
| --- | --- |
| `domain.env` | `TEST_DOMAIN`, nameservers, `VM_PUBLIC_IPV4` published in DNS / `public.env` |
| `smtp.env` + `smtp_password` | Postfix SASL relay (`relayhost`) when port 25 is blocked |
| `vm.env` | SSH target for remote Ubuntu 24.04 acceptance (`scripts/remote-vm.sh`) |
| `server-ssh-key_rsa.prv` | Optional; required for `remote-vm.sh` (not shipped) |

Load into the current shell:

```bash
source scripts/load-validation-env.sh
```

The installer reads `PANEL_VALIDATION_DIR` (default: this directory, then `/var/lib/panel/validation`). See `docs/validation.md`.
