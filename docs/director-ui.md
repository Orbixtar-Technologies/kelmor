# Kelmor Director operator console

Kelmor Director is the provider-facing console for shared-hosting operators.
Tenants use Kelmor Control. Director follows familiar server-panel information
architecture while using Kelmor branding, vocabulary, APIs, and privilege
boundaries exclusively.

For the route-by-route WHM pattern mapping, see
[`director-whm-journey-checklist.md`](director-whm-journey-checklist.md).

## Chrome

- A light workspace with clear borders, compact controls, and blue actions.
- A persistent Kelmor Director identity, live hostname, notifications, and
  administrator menu.
- Global Find searches capability-available tools, usernames, and domains.
  Press `/` to focus it, use arrow keys to select, and press Enter to navigate.
- The left rail groups tools by operator task. Categories can be expanded,
  collapsed, and filtered, and the rail can be collapsed on desktop.
- Mobile navigation is inert while closed, moves focus into the drawer when
  opened, closes with Escape, and restores focus to its trigger.
- Breadcrumbs preserve location context throughout the application.

Tool discovery is capability-aware, but the API remains the security boundary.
Compound journeys require every capability needed by their route and data
sources.

## Home

Home combines:

- Favorite high-frequency tools.
- Load, memory, disk, account, and job statistics.
- Service status and recent jobs.
- Recent privileged activity when the role can read audit data.
- All available tools grouped by category.

Firewall and reboot are intentionally secondary controls under Security & Host
Configuration.

## Operator journeys

### Account Information and Account Functions

- List Accounts provides search, sorting, pagination, suspended and over-quota
  views, package filtering, selection, and reviewed suspend actions.
- Create Account is a three-step identity, package/ownership, and exact-payload
  review wizard.
- Account Summary combines observed state, usage, package and reseller
  assignment, login controls, related jobs, and reviewed lifecycle actions.
- Password rotation can require the owner to choose a different password in
  Kelmor Control before another session is issued.
- Account state and reconciliation jobs are written atomically. Concurrent
  stale updates return a conflict instead of overwriting newer state.

### Account-linked services

Account Services groups websites, domains, applications, SQL databases, mail
domains, mailboxes, aliases, certificates, bounded files, backups, cron jobs,
SSH/SFTP, FTP, and account API tokens. Dedicated WHM-style hubs also exist for
List Domains, MultiPHP Manager, Cron Jobs, FTP Accounts, Email Deliverability,
Feature Manager, and Process Manager. DNS Management supplies multi-zone record
and DNSSEC operations. Login to Kelmor Control uses reasoned impersonation.

Requests are scoped to the selected account. Response sequencing prevents a
slower request for a previous account or zone from replacing the current view.

### Packages and Resellers

- Packages exposes all resource limits, assignment counts, create/edit, and
  safe delete. Package deletion remains blocked while any account references
  it.
- Resellers exposes identity, status, nameservers, and an enforced privilege
  mask bounded by the reseller role.
- Historical reseller rows receive their previous default privilege set during
  migration; a new explicit empty mask means no delegated capabilities.

### Status, Security, Transfers, Jobs, and Audit

- Server & Service Status shows measured host vitals, managed service probes,
  and a truthful bounded view of the current control-plane process.
- Security & Host Configuration provides reviewed firewall apply and typed
  `REBOOT` confirmation flows.
- Transfers & Backups supports native export, server-scoped import, reviewed
  account copy/migration, and links to account backup/restore histories.
- Jobs provides filters, pagination, redacted payload detail, logs, eligible
  failed-job retry, and secure cancellation through the API.
- Audit provides text/action/outcome/date filters, pagination, and full
  before/after/metadata details.

## Backend endpoints

Director consumes the same capability-enforced API as the CLI. The merged API
surface includes:

| Endpoint | Purpose |
| --- | --- |
| `GET/PUT/PATCH/DELETE /packages/{id}` | Package detail, validated replacement/update, and atomic safe delete |
| `GET /feature-sets` | Package feature-set discovery |
| `GET/PATCH /resellers/{id}` | Server-scoped detail and validated reseller changes |
| `POST /accounts/{id}/password` | Atomic owner and Linux-password reconciliation |
| `POST /auth/complete-password-change` | Required owner password completion before login |
| `POST /jobs/{id}/retry` | Clone an eligible failed job while retaining immutable history |
| `POST /jobs/{id}/cancel` | Atomically cancel an authorized queued or failed job |
| `GET /audit-events` | Filtered and paginated privileged activity |
| `GET /server/dns/zones` | Server-wide zone index with account and record context |
| `GET /accounts` | Account, package, reseller, over-quota filters and scoped usage |

Job payloads are recursively credential-redacted. Account tokens are bound to
one account and may contain only account-safe capabilities already held by the
creator. Native host-path imports require server scope.

## Local verification

```bash
make lint
make test
make test-security
npm test --prefix portals/server
npm run build --prefix portals/server
```

Development endpoints remain:

- Kelmor API: `http://127.0.0.1:18080`
- Kelmor Director: `http://127.0.0.1:18443`
- Kelmor Control: `http://127.0.0.1:18444`
