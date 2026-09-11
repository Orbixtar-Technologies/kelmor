# Kelmor Director operator journey checklist

This checklist maps familiar WHM operator patterns to Kelmor Director without
copying WHM branding, proprietary assets, or implementation details. Kelmor's
API capability checks and typed privileged-agent boundary remain authoritative.

## Chrome and Home

| Operator pattern | Kelmor route | Implementation |
| --- | --- | --- |
| Global feature and account search | All routes | Top-bar Find searches tools, usernames, and domains; `/` focuses it. |
| Categorized feature navigation | All routes | Collapsible, filterable sidebar generated from capability-aware tool metadata. |
| Host identity and administrator controls | All routes | Live hostname, notifications, Kelmor Director identity, administrator menu, and sign out. |
| Home favorites and server status | `/` | Favorites, load/memory/disk/account/job stats, services, recent jobs, activity, and grouped tools. |

## Account Information and Account Functions

| Journey | Kelmor route | API |
| --- | --- | --- |
| List and search accounts | `/accounts` | `GET /api/v1/accounts` |
| Suspended accounts | `/accounts?view=suspended` | Account inventory status filter |
| Over-quota accounts | `/accounts?view=over-quota` | Account inventory joined with package limits and measured usage |
| List addon, parked, and subdomains | `/domains` | Per-account `GET /api/v1/accounts/:id/domains` |
| Account summary | `/accounts/:id` | Account, package, reseller, usage, and related-job APIs |
| Login to Kelmor Control | Account list and summary | Reasoned `POST /api/v1/accounts/:id/impersonate` |
| Create account | `/accounts/create` | Reviewed three-step wizard → `POST /api/v1/accounts` |
| Modify account and change package | `/accounts/:id` | `PATCH /api/v1/accounts/:id` |
| Suspend or unsuspend | Account list and summary | `POST /suspend` or `POST /unsuspend` |
| Force password change | `/accounts/:id` | `POST /password`, owner login hash update, and queued Linux reconciliation |
| Terminate account | `/accounts/:id` | Username-confirmed `POST /terminate` |
| Bulk suspend | `/accounts` | `POST /api/v1/accounts/bulk/suspend` |
| Bulk unsuspend | `/accounts` | `POST /api/v1/accounts/bulk/unsuspend` |

## Packages and Resellers

| Journey | Kelmor route | API |
| --- | --- | --- |
| Add/edit/delete packages | `/packages` | `GET`, `POST`, `PATCH`, and safe `DELETE /api/v1/packages` |
| Feature Manager | `/features` | `GET /api/v1/feature-sets` plus package assignment |
| Rich account limits | `/packages` | Disk, bandwidth, domains, databases, mail, access, backup, CPU, memory, process, I/O, and request limits |
| List/create/edit resellers | `/resellers` | `GET`, `POST`, and `PATCH /api/v1/resellers` |
| Reseller privileges | `/resellers` | Editable privilege mask, status, nameservers, and brand metadata |

## Account-linked services

Select an account through global Find, List Accounts, or one of the dedicated
tool entries. Account operations remain scoped to `/accounts/:id/services`.

| Tool family | Real Kelmor APIs |
| --- | --- |
| Domains and websites | Domain, website, application, and WordPress operations. Dedicated `/domains` and `/websites` (MultiPHP) hubs. |
| DNS | Zones, records, DNSSEC, and DS record reads |
| SQL | MariaDB/MySQL/PostgreSQL database operations |
| Email | Mail domains, catch-all routing, mailboxes, aliases, and `/deliverability` SPF/DKIM/DMARC checks |
| Cron and FTP | Dedicated `/cron` and `/ftp` hubs using existing account APIs |
| SSL/TLS | Account-owned certificate list and request |
| Files and access | Bounded files, SSH keys, SFTP password, FTP users, and API tokens |
| Backups | Encrypted local/SFTP/S3 backup creation, history, and reviewed restore |
| Automation | Cron jobs and related durable jobs |
| Usage | Disk, bandwidth, inodes, memory, process, and CPU observations |

## Server, Security, Transfers, and History

| Journey | Kelmor route | Implementation |
| --- | --- | --- |
| Server and service status | `/status` | Measured vitals plus service and process observations |
| Process Manager | `/processes` | Bounded `GET /api/v1/server/processes` snapshot |
| Firewall | `/security` | Read current Kelmor firewall metadata, review, confirm, and apply through the privileged agent |
| Host reboot | `/security` | Typed `REBOOT` confirmation through the privileged agent |
| Native transfer | `/transfers` | Native export/import with override review |
| Extracted archive import | `/transfers` | Protected host-path import review |
| Account copy/migration | `/transfers` | Existing-account export/import and home-directory copy |
| Job history and retry | `/jobs` | State/search/account filters, pagination, safe payload detail, logs, and capability-aware failed-job retry |
| Audit history | `/audit` | Search, action/outcome/date filters, pagination, and before/after/metadata detail |

## Backend status

- Package update/delete, reseller update, owner password rotation, and failed-job
  retry are real capability-gated API operations.
- Forced password rotation is atomic with its reconciliation job. Kelmor Control
  requires the owner to choose a different password before issuing a session.
- Account API tokens are bound to the selected account and restricted to the
  actor's account-safe capabilities.
- Native and extracted host-path imports require server scope; reseller
  migrations use the reviewed account-copy journey.
- Account lifecycle writes use atomic state-and-job transactions and reject
  stale reviewed revisions.
- Host-impacting changes continue through API → durable job or typed agent.
- Job list/detail payloads recursively redact password, secret, token,
  credential, key, and hash fields.
- Service status and process lists use the existing lightweight host probes.
  Unsupported arbitrary service mutation is not exposed.
- Package deletion fails while any account references the package.
- Failed-job retry preserves immutable history and creates a new operation.

## Verification

Run:

```bash
npm test --prefix portals/server
npm run build --prefix portals/server
go test ./internal/httpserver ./internal/store ./internal/job
make lint
make test
make test-security
```

Browser verification should cover Home, global Find, List Accounts, account
creation review, account summary/services, Packages, Resellers, DNS, Service
Status, Security, Transfers, Jobs, Audit, Usage, and narrow-screen navigation.
