# Kelmor Director — operator console

Director is the provider-facing console. It is built for someone who administers a
shared hosting server for a living: the person who spends their day creating accounts,
moving customers between plans, chasing a failed job and answering "why is this site
down". Tenants never see Director; they use Kelmor Control.

The information architecture deliberately follows the conventions that shared-hosting
operators already know — a persistent top bar with a global find, a left rail of
collapsible feature categories, and a home page of favourites, statistics, service
health and category tool tiles. Everything is Kelmor branding, Kelmor vocabulary and
Kelmor APIs. No third-party marks, stylesheets or assets are used.

## Chrome

| Element | Behaviour |
| --- | --- |
| Brand mark | `K` monogram plus the "Kelmor Director" wordmark, always links home |
| Global find | `/` focuses it. Matches feature tools **and** accounts by username or domain. Arrow keys move, Enter navigates |
| Notifications | Failed jobs and suspended accounts, each linking to the tool that resolves it |
| Host chip | The hostname reported by the privileged agent, so an operator with several tabs knows which node they are on |
| User menu | Signed-in username, roles, sign out |
| Category rail | Categories start minimized. Expand all / Collapse all, plus a filter box that searches tool names and keywords and temporarily expands whatever matches. The category holding the current route opens itself |
| Breadcrumbs | Home › Category › Tool on every page |
| Favourites | The star beside any page title pins that tool to the Home favourites panel. Stored per browser |

Every tool the current role cannot use is hidden from the rail, from Home and from
global find, and its route renders a "your role does not include this capability"
page. The control plane enforces the same check on every request — Director hiding a
tool is a convenience, never the security boundary.

## Feature categories

Director groups tools the way an operator thinks about the job. The table below maps
each classic shared-hosting operator task to the Kelmor tool that performs it, and
notes where Kelmor deliberately does something different.

### Account Functions

| Task | Kelmor tool | Notes |
| --- | --- | --- |
| Create a new account | **Create a New Account** | Four-step wizard: identity → package and owner → networking → review, then a queued `account.provision` job |
| Modify an account | **Modify an Account** | Primary domain, dedicated IP, reseller owner, interactive-login state |
| Upgrade / downgrade | **Upgrade / Downgrade an Account** | Side-by-side old/new limit comparison, and a warning when the new disk limit is below current usage |
| Suspend / unsuspend | **Manage Account Suspension** | Multi-select, one confirmation for the whole batch |
| Terminate | **Terminate Accounts** | Multi-select behind a typed `TERMINATE` confirmation |
| Force password change | **Force Password Change** | Rotates the owner password and the SFTP credential, optionally requiring a change at next sign-in |
| Limit bandwidth / disk | **Limit Bandwidth and Disk** | Kelmor enforces limits from the *package*, not per account. This tool shows consumption against the cap and routes to the package or to a package change |

### Account Information

| Task | Kelmor tool |
| --- | --- |
| List accounts | **List Accounts** — search, sortable columns, pagination, per-row action menu, bulk suspend |
| List suspended accounts | **List Suspended Accounts** |
| Show accounts over quota | **Show Accounts Over Quota** — served by the `over_quota` API filter |
| Account summary | **Account Summary** picker, and the per-account hub at `/accounts/{id}` with nine tabs: overview, domains and sites, DNS, email, databases, SSL, files, backups, jobs |

### Packages, Resellers

| Task | Kelmor tool |
| --- | --- |
| Add / edit / delete a package | **Packages**, **Add a Package**, package editor. Deleting a package that accounts still use is refused by the API with `PACKAGE_IN_USE` |
| Feature manager | **Feature Manager** — read-only view of the feature sets packages point at |
| Create / edit resellers, privileges, ownership | **Resellers**, **Add a Reseller**, reseller detail with a privilege mask bounded by the reseller role, plus the accounts and packages the reseller owns |

### DNS, SQL, Email, SSL

| Task | Kelmor tool |
| --- | --- |
| Zone list and record editing | **DNS Zone Manager** and the per-zone record editor |
| DNSSEC and DS records | **DNSSEC**, and the signing panel on each zone |
| Add a zone | **Add a DNS Zone** — explains that zones are derived from accounts, and points at the tool that actually creates one |
| Databases | **Databases** — every MariaDB and PostgreSQL database on the host, grouped by owning account |
| Mail domains, mailboxes, aliases | **Mail Domains and Mailboxes** |
| Certificates | **Certificates** — issuance, issuer, remaining lifetime, expiry warnings, new ACME orders |

### Server Status, Security, Backup and Transfers, Jobs, Statistics

| Task | Kelmor tool |
| --- | --- |
| Service status | **Service Status** — observed health per managed service |
| Server information | **Host Vitals** — load, memory, disk, inodes, uptime, hosting totals |
| Process manager | **Process Manager** — read-only; see "deliberate differences" below |
| Firewall | **Host Firewall** — review the policy, apply it behind a confirmation |
| Graceful server reboot | **Graceful Server Reboot** — demoted into Server Configuration, behind a typed `REBOOT` confirmation and a pre-flight checklist |
| Audit log | **Audit Log** — filter by search, action, resource type, outcome and date range, paged, with a full before/after detail drawer |
| Backups | **Account Backups** — queue encrypted runs, review every run on the server |
| Restore | **Restore a Backup** — typed `RESTORE` confirmation |
| Transfer / migrate | **Transfer or Migrate an Account** |
| Import | **Import an Account** — Kelmor native export, or an extracted cPanel `cpmove` tree |
| Export | **Export Accounts** |
| Job queue | **Job Queue** — state and type filters, clickable state chips, pagination, retry, cancel, and a detail drawer with the payload (credentials redacted) and worker log |
| Bandwidth and disk usage | **Bandwidth and Disk Usage** |

## Deliberate differences

These are places where Director does *not* copy the conventional shared-hosting panel,
because the underlying architecture is different and pretending otherwise would mislead
the operator.

- **Limits live on packages.** There is no per-account quota editor. Changing one
  account means moving it to another package; changing a plan means editing the package.
  The quota tool says so and links to both.
- **Zones are derived from accounts.** There is no standalone "add zone" form, because a
  zone with no hosting account behind it would drift away from the vhost and mail
  routing it is supposed to match. Add a DNS Zone documents the real path.
- **No process kill.** Runaway workloads are contained by the CPU, memory, process and
  I/O limits written to each account systemd slice, not by signals sent from a browser.
- **No arbitrary shell.** Every privileged action is a typed operation dispatched to the
  agent and recorded as a durable job.
- **Reboot is demoted.** It sits in Server Configuration behind a checklist and a typed
  confirmation, rather than as a button on the home dashboard.

## Destructive actions

Everything that loses data or takes a site off the air goes through a confirmation
dialog that names the blast radius. Three of them require the operator to type a
literal word first: account termination (`TERMINATE`, or the username on the account
hub), backup restore (`RESTORE`) and host reboot (`REBOOT`).

## Backend endpoints added for these journeys

Director is a client of the same public API as the CLI. These endpoints were added so
the operator journeys above are complete rather than partially wired:

| Endpoint | Purpose |
| --- | --- |
| `GET/PUT/DELETE /packages/{id}` | Package read, full-record update, and delete guarded against in-use packages |
| `GET /feature-sets` | Feature Manager |
| `GET/PATCH /resellers/{id}` | Reseller detail with owned accounts and packages; privilege-mask update validated against the reseller role |
| `POST /accounts/{id}/password` | Force Password Change |
| `POST /jobs/{id}/retry`, `POST /jobs/{id}/cancel` | Job queue recovery |
| `GET /audit-events` filters | `q`, `action`, `resource_type`, `account_id`, `actor_id`, `success`, `since`, `until`, `limit`, `offset`, returning `total` |
| `GET /server/dns/zones` | Server-wide zone index joined with the owning account and record count |
| `GET /accounts` filters | `package_id`, `reseller_id`, `over_quota`, plus a `usage` map so the accounts table renders disk and transfer without a request per row |

## Running Director locally

```bash
createdb panel_control
PANEL_DEV=1 PANEL_DATABASE_URL=postgres:///panel_control?host=/var/run/postgresql \
  PANEL_STATE_DIR=$PWD/var/panel ./dist/bin/panel-dev     # API on :18080
npm run dev --prefix portals/server                        # Director on :18443
```

Sign in as `admin` / `ChangeMeOnce!2026`. Change that password before any real host.
