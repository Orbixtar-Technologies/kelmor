# Kelmor portals vs a normal hosting panel

**Tree:** `origin/main` at `2f83c25` (19 September 2026).  
**Scope:** Kelmor Director (`portals/server`) and Kelmor Control (`portals/account`) plus the API and typed Agent they call.  
**Not in scope:** implementing the Control rewrite, copying WHM/cPanel trademarks or chrome, billing, Windows, or Kubernetes.

This is an honest surface-and-status map. It exists because tenant and operator feedback is that Email, Database, File Manager, and the rest of the portal **do not feel like a normal hosting panel**. Director already has a WHM-shaped catalog. Control is still a thin account SPA. Many catalog tiles look complete and only persist a JSON preference file.

---

## 1. Verdict

Kelmor’s **backend** already covers the shared-hosting operating loop: POSIX account, nginx/php-fpm, PowerDNS, MariaDB/PostgreSQL, Postfix/Dovecot, ACME, bounded files, SFTP/FTP, cron, HPM1 backup/restore. Those paths go API → durable job → typed Kelmor Agent. That is real.

The **portals** do not present that loop the way a normal panel does:

| Surface | Analog | What operators/tenants get today |
| --- | --- | --- |
| **Kelmor Director** | WHM | Light WHM-style chrome, categorized catalog, real P0 account/job/host journeys. Dozens of sidebar tools are **settings-only** (`PATCH /api/v1/server/settings` → `director-settings.json`) or honest “Kelmor equivalent” status pages. Dedicated managers are the usable product. |
| **Kelmor Control** | cPanel | Seven nav items, serif/cream chrome, inline forms. Most pages call real account APIs, but journeys are **create/list/delete** — not manage. File Manager cannot delete, mkdir, rename, chmod, or upload even though those APIs exist. Email has no webmail, no mailbox password rotate, no lists. Databases have no credentials and no phpMyAdmin. Account has no voluntary password or contact page. |

The mismatch is therefore **information architecture and journey completeness**, not a missing control plane. Plan v3 says: make Control cPanel-class on six P0 families first, and keep honest stubs only when they are labeled.

---

## 2. Status words

| Status | Meaning |
| --- | --- |
| **API-backed** | UI calls a capability-gated `/api/v1` route that persists desired state and/or queues a job / typed Agent method. |
| **Partial journey** | API-backed create/list exists; everyday manage actions (edit, password, credentials, delete, launch) are missing or only on the other portal. |
| **Settings stub** | Save writes `GET`/`PATCH /api/v1/server/settings`. Nothing in the Agent or provision path consumes that key. |
| **Honest status** | Page explains the Kelmor equivalent and does not pretend to mutate the host (or only launches a real related tool). |
| **Missing** | No Control/Director route *and* no API (or API would be a new product). |
| **Broken / misleading** | UI copy or layout implies a host change that cannot happen, or a required API exists and this portal does not call it. |

---

## 3. Approved plan v3 (product contract for the next rewrite)

In-repo written plan [`docs/plans/2026-09-09-001-feat-whm-style-kelmor-director-plan.md`](../plans/2026-09-09-001-feat-whm-style-kelmor-director-plan.md) shipped Director P0 and **explicitly left Control redesign out of scope**.

**Plan v3** (approved direction for this audit) is:

1. **Control P0 = cPanel-class** on six families: **Domains/DNS, Files, Email, DB, SSL, Account**.
2. Each P0 family must be a first-class Control place with search, dense lists, create, and **manage** — not a single dump of forms.
3. Reuse existing `/api/v1/accounts/{id}/…` endpoints and Agent methods. Do not invent a second control plane.
4. **Honest stubs are allowed only if labeled** (“saves a Director preference; does not apply on the host”). Unlabeled settings saves that look like WHM tools are a defect.
5. Kelmor branding only. No WHM/cPanel trademarks, logos, or copied assets.
6. Do not flatten privilege zones: unprivileged API/portals, root-owned typed Agent, tenant POSIX identity.

Director work after v3 is **honesty and remaining WHM-shaped gaps**, not another catalog dump.

---

## 4. How it works today

```text
Browser (Director :2087 / :18443 or Control :2083 / :18444)
    → Kelmor API (capability + account membership)
        → PostgreSQL desired state + durable jobs
            → Kelmor Agent (/run/panel/agent.sock, allow-listed methods)
                → nginx, php-fpm, pdns, postfix, dovecot, mariadb, vsftpd, files
```

### 4.1 Account context

| Rule | Director | Control |
| --- | --- | --- |
| Who is in scope | Operator picks an account (`AccountPicker`, `?account=`, `/accounts/:id`). First inventory row is auto-selected if the URL has no id. | Bound to `me.actor.account_ids[0]` only. No picker. Extra memberships are invisible. |
| Empty account | Tools show an account picker empty state. | “No hosting account is attached to this login yet.” |
| Cross-account | Global Find + List Accounts + impersonation (`POST /accounts/{id}/impersonate`) opens Control as the owner. | Tenant session only. No host reboot, firewall, or package edit. |
| Sequencing | `RequestSequence` drops stale account/zone responses on dedicated managers. | Same helper on files/DNS/websites. Inline Email/DB/Cron in `app.tsx` do not all use it. |

### 4.2 Agent operations tenants actually need

Typed methods (not a shell) that back the P0 families include: `ApplyWebsite`, `ApplyPhpPool`, `ApplyDNSZone`, `SetDNSSEC`, `CreateHostedDatabase` / `DropHostedDatabase`, `ApplyMailMaps`, `CreateMailboxHome`, `EnsureDKIM`, `IssueDevCertificate` / ACME path, `ListDirectory`, `ReadManagedFile`, `ApplyFile`, `DeleteManagedFile`, `RenameManagedPath`, `ChmodManagedPath`, `ApplyFTPUsers`, `ApplyAuthorizedKeys`, `SetLinuxPassword`, `ApplyAccountCron`, `PackDirectory` / backup restore, `ApplyAdminTools` (phpMyAdmin / Roundcube vhosts).

Portals must never write `/etc`, nginx, or passwd maps themselves.

### 4.3 Known platform limits (honest, not UI bugs)

- One **auto-provisioned database user** per engine/account (credential file under the home). Not cPanel’s many MySQL users + GRANTs.
- Mailbox **password/quota change has no API** (`POST` create + `DELETE` only). Control and Director cannot offer rotate-password until that exists.
- Mailing lists are **Postfix multi-member aliases**, not GNU Mailman.
- Node/Python `ApplyAppUnit` and WordPress exist; they are post-MVP depth, not P0 Control.
- Public TLS needs a routable A/AAAA and :80. Lab/QEMU uses Pebble.
- `GET`/`PATCH /server/settings` is a host-local JSON blob. It is not desired state for nginx, Postfix, or PowerDNS unless a later worker learns that key.
- Control login form still prefills lab username/password (`livehost` / `TenantPass!2026`) in `portals/account/src/app.tsx`. Harmless on a closed lab; wrong for a tenant-facing product.

### 4.4 Privilege and branding

Director and Control hide nav by `/api/v1/me` capabilities; the API still enforces them. Reseller visibility is membership-scoped. Job payloads redact secrets. Product names are **Kelmor Director**, **Kelmor Control**, **Kelmor Agent** only.

---

## 5. Surface map

### 5.1 Kelmor Control — routes and tools

Chrome: `portals/account/src/app.tsx` + `control-hubs.ts`. Flat left nav, green/cream theme, no Find, no breadcrumbs, no icons, no notifications.

| Nav / route | Tabs | Page module | Tenant tools |
| --- | --- | --- | --- |
| `/` Dashboard | — | inline `Dash` | Primary domain, status, home, UID, disk + monthly transfer |
| `/websites` | Sites, SSL/TLS | `pages/websites-page.tsx`, inline `Certificates` | Apply site/runtime/PHP, WordPress install, cert list + request |
| `/domains` | Domains, DNS | inline `Domains`, `pages/dns-page.tsx` | Addon/subdomain/alias attach + remove; zone select; add A/AAAA/CNAME/MX/TXT |
| `/email` | — | inline `Email` | Create/delete mailbox, catch-all, aliases |
| `/databases` | — | inline `Databases` | Create/delete MariaDB or PostgreSQL name |
| `/files` | — | `pages/files-page.tsx` | Browse, read/write text, SFTP password, SSH keys, FTP users |
| `/backups` | Backups, Cron | `pages/backups-page.tsx`, inline `Cron` | Full backup (local/SFTP/S3), restore; cron add/remove |
| `/ssl` | redirect | → `/websites?tab=ssl` | — |
| `/dns` | redirect | → `/domains?tab=dns` | — |
| `/cron` | redirect | → `/backups?tab=cron` | — |
| Login only | — | `password-change-form.tsx` | Forced password completion (`PASSWORD_CHANGE_REQUIRED`) |

**Not in Control nav at all:** Account settings, webmail, phpMyAdmin, mailing lists, deliverability, FTP as its own app, applications, API tokens, PHP INI, redirects, metrics, error pages, hotlink, IP deny, autoresponders, spam filters, custom certificate upload.

### 5.2 Kelmor Director — dedicated routes

Chrome: `layout/director-shell.tsx`, categorized sidebar from `whm-catalog.ts` via `nav-hubs.ts` / `operator-nav.ts`. Global Find. Dedicated managers stay free of hub-tab dumps (`shouldShowHubTabs` is `/section/:hub` landings only; `?tool=` catalogs stay hidden).

| Route | Page | Analog WHM family |
| --- | --- | --- |
| `/` | `home-page.tsx` | WHM Home (vitals + catalog) |
| `/accounts`, `/accounts/create`, `/accounts/:id` | list / wizard / summary | Account Information + Functions |
| `/accounts/:id/services` | `account-services-page.tsx` | Per-account service dump |
| `/packages`, `/features`, `/resellers` | packages / feature sets / resellers | Packages, Feature Manager, Resellers |
| `/domains` | `domains-page.tsx` | List domains / addon / parked / sub |
| `/websites` | `websites-page.tsx` | MultiPHP Manager |
| `/dns` | `dns-page.tsx` | DNS Zone Manager (edit, duplicate, delete, DNSSEC) |
| `/files` | `file-manager-page.tsx` | File Manager |
| `/ftp` | `ftp-page.tsx` | FTP Accounts |
| `/cron` | `cron-page.tsx` | Cron Jobs |
| `/sql` | `sql-manager-page.tsx` | Database / MySQL / pg |
| `/email` | `email-manager-page.tsx` | Email Accounts + lists |
| `/deliverability` | `deliverability-page.tsx` | Email Deliverability |
| `/webmail` | `webmail-page.tsx` | Webmail + client settings |
| `/ssl` | `ssl-manager-page.tsx` | SSL/TLS + AutoSSL |
| `/processes` | `process-manager-page.tsx` | Process Manager |
| `/status` | `service-status-page.tsx` | Service Status |
| `/security` | `security-page.tsx` | Security + reboot |
| `/transfers`, `/import` | `transfers-page.tsx` | Transfers / restore |
| `/jobs` | `jobs-page.tsx` | Task queue |
| `/updates` | `updates-page.tsx` | Software Updates |
| `/audit` | `audit-page.tsx` | Audit Trail |
| `/usage`, `/monitor` | `usage-page.tsx` | Bandwidth / disk usage |
| `/section/:hubId` | `hub-page.tsx` | Category landing + generic tool |
| `/tools/:toolId` | `whm-tool-page.tsx` | Generic WHM-named journey |

### 5.3 Director catalog (sidebar completeness)

`whm-catalog.ts` lists **every familiar WHM category** (Account Functions, DNS Functions, Email, SQL Services, SSL/TLS, Files, Software, Backup, Security Center, and the rest). About **76** entries are `dedicated: true` (they open a real manager). About **77** carry a `settingKey` and save the JSON blob.

Generic apply logic lives in `applyFeature` (`whm-tool-page.tsx`):

| Catalog layout | What Save actually does |
| --- | --- |
| `restart` | `POST /server/services/{name}/restart` — **API-backed** |
| `account-action` patch/suspend/password/terminate | Account APIs — **API-backed** when the field maps to `ip_address`, `package_id`, `shell_class`, or a dedicated password/suspend route |
| `wp-toolkit` | `POST /accounts/{id}/wordpress` — **API-backed** |
| `easyapache` | `POST /server/runtimes` — **API-backed** |
| root / MariaDB root password | `POST /server/root-password` or `/server/database-root-password` — **API-backed** |
| `phpmyadmin` / `market` / `plugins` / `terminal` / mail-queue | Host app or recipe panels — **API-backed** or **honest status** |
| anything with `settingKey` else | `PATCH /server/settings` — **settings stub** |
| leftover `confirm` / `wizard` / `form` | same settings blob keyed by feature id — **settings stub** |

Quota Modification and Limit Bandwidth Usage are the sharp examples: copy says the Agent enforces a per-account cap, but Save does **not** `PATCH` `disk_bytes` / `bandwidth_bytes_monthly` on the account. It stores a preference. Treat as **misleading** until labeled or wired.

---

## 6. Implementation status by family

### 6.1 Shared hosting families (Email, DB, Files, DNS, SSL, …)

| Family | Control | Director | API / Agent | Status |
| --- | --- | --- | --- | --- |
| **Account (tenant identity)** | Dashboard read-only. Forced password at login only. No contact, 2FA, language, or voluntary rotate. First `account_ids` only. | Full list/create/modify/suspend/terminate/password/impersonate. | `GET/PATCH /accounts/{id}`, `/password`, `/usage`, `/impersonate` | Control **partial**. Director **API-backed**. |
| **Domains** | Attach addon/subdomain/alias; remove non-primary. | Same APIs, denser table, type filters. | `GET/POST/DELETE /domains` | **API-backed**. Control UX is a form + `<ul>`. |
| **DNS** | Zone picker, add record, list. **No delete, edit, DNSSEC, DS.** | Add/edit/duplicate/delete + DNSSEC + DS. | `…/dns/zones`, records POST/DELETE, `/dnssec`, `/ds`. No record PATCH (edit = delete+add). | Control **partial / broken manage**. Director **API-backed**. |
| **Websites / PHP** | Apply runtime + PHP 8.3/8.4/8.5 select; WordPress form. No php.ini, no docroot picker beyond apply, no redirects. | MultiPHP manager + catalog INI **settings stub**. EasyApache runtimes **API-backed**. | `POST /websites`, `POST /wordpress`, `POST /server/runtimes` | **API-backed** for apply. INI **settings stub**. |
| **SSL / TLS** | Tab: list + request hostname. No expiry urgency, SAN, custom paste, AutoSSL policy. | Dedicated `/ssl` inventory / request / status / AutoSSL task. | `GET/POST /certificates` (ACME). No custom PEM upload API. | **API-backed** issue. Custom cert **missing**. Control **partial**. |
| **Email** | Mailbox create/delete, catch-all, aliases. No lists, webmail, client settings, filters, autoresponders, quota edit, password rotate. | `/email` tabs + `/webmail` + `/deliverability`. Still no mailbox PATCH. | mail domains PATCH; mailboxes POST/DELETE; aliases; lists CRUD; DKIM via provision | **Partial** both UIs. Password/quota **missing API**. Control **missing** lists/webmail. |
| **Databases** | Name + engine create/delete. No user, host, privileges, credentials, dump, phpMyAdmin. | `/sql` + connection panel + admin-tools URL. | `GET/POST/DELETE /databases`, `GET …/credentials`, `GET …/admin-tools` | Control **partial / broken manage**. Director **API-backed** for the Kelmor model (one user). |
| **Files** | List, open text, write path. **No mkdir/delete/rename/chmod/upload.** FTP + SSH mixed on the same page. | List, search, sort, paginate, upload, mkdir, rename, delete, protected-path warning. | `GET/POST /files`, `/content`, `POST /mkdir`, `DELETE /files`, `PATCH /files` | Control **broken journey** vs existing API. Director **API-backed**. |
| **FTP** | Create/remove virtual users on Files page. No dedicated app, no directory picker, no quota. | Dedicated `/ftp`. | `GET/POST/DELETE /ftp` → `ApplyFTPUsers` | **API-backed**. Control **partial**. |
| **SFTP / SSH** | Set SFTP password; add/remove keys. | Account services + file-adjacent tools. | `/sftp-password`, `/ssh-keys` | **API-backed**. |
| **Cron** | Add/remove on Backups tab. No edit, no email-on-fail, no disabled toggle in UI (`enabled: true` hardcoded). | Dedicated `/cron`. | `GET/POST/DELETE /cron` | **API-backed**. Control **partial**. |
| **Backups** | Full backup + in-place restore. Polls list. | Transfers + account services + backup **settings stubs** (config, user selection). | `POST /backups`, `POST /restores`, export/import | **API-backed** for run/restore. Policy pages **settings stub**. |
| **Webmail** | **Missing** (no call to `admin-tools`). | `/webmail` + IMAP/SMTP copy + open URL. | `GET /accounts/{id}/admin-tools` | Control **missing**. Director **API-backed** when Roundcube is published. |
| **phpMyAdmin** | **Missing**. | `/tools/phpmyadmin` + SQL page link. | `POST /server/apps/{id}/enable`, admin-tools URL | Control **missing**. Director **API-backed**. |
| **Deliverability** | **Missing**. | `/deliverability` SPF/DKIM/DMARC. | DNS reads + DKIM Agent | Control **missing**. Director **API-backed**. |
| **Applications** | WordPress only, on Websites. No Node/Python manager. | Account services + WP toolkit tool. | `/applications`, `/wordpress` | **Partial**. |
| **API tokens** | **Missing**. | Account services. | `/api-tokens` | Control **missing**. |
| **Usage** | Dashboard bytes only. | `/usage` dense compare. | `GET /accounts/{id}/usage` | Control **partial**. |
| **Account API password** | Forced change only. | Rotate from summary. | `POST /accounts/{id}/password`, `POST /auth/complete-password-change` | Control **partial**. |

### 6.2 Director-only (WHM) families

| Family | Route / tool | Status |
| --- | --- | --- |
| List / create / lifecycle accounts | `/accounts*` | **API-backed** |
| Packages + feature sets | `/packages`, `/features` | **API-backed** |
| Resellers + privilege mask | `/resellers` | **API-backed** |
| Jobs + retry/cancel | `/jobs` | **API-backed** |
| Audit | `/audit` | **API-backed** |
| Service status + control | `/status`, `/tools/restart-*` | **API-backed** |
| Process snapshot + signal | `/processes` | **API-backed** |
| Firewall apply + typed reboot | `/security` | **API-backed** |
| Native transfer / cPanel tree import | `/transfers` | **API-backed** (importer emits native export; not cPanel binaries) |
| Software updates | `/updates` | **API-backed** |
| Host recipes / “terminal” | `/tools/terminal` | **API-backed** (allow-listed recipes only) |
| PHP runtime install | `/tools/easyapache` | **API-backed** |
| Cluster, IP pool, greylist, 2FA policy, tweak settings, theme, locales, Market extras, module installers, … | `/tools/*` + `settingKey` | **Settings stub** unless listed above |
| In-browser root shell | — | **Missing** on purpose |
| Billing / WHMCS | — | **Missing** on purpose |

---

## 7. Gap vs a normal hosting panel

Comparisons are **operator-journey** comparisons. Kelmor must not copy WHM or cPanel trademarks, layout assets, or proprietary copy.

### 7.1 Director vs WHM

WHM administrators expect: categorized Find, account list → summary → linked Email/SQL/DNS/SSL, package/reseller, service status, transfers, jobs.

| Expectation | Kelmor Director |
| --- | --- |
| Home + Find + categories | Present. Dedicated P0 pages are real. |
| Create / list / suspend / terminate | Present and reviewed. |
| Account-linked Email/SQL/DNS/SSL/Files | Present as dedicated managers with `?account=`. |
| Every sidebar name does the named host action | **No.** Catalog completeness outran apply completeness. Many tiles are preference forms. |
| Tweak Settings / EasyApache / Exim / Mailman / Market | Honest Kelmor stand-ins or stubs. EasyApache installs php-fpm versions. Mailman → Postfix lists. Exim → Postfix settings blob. |
| Cluster DNS, remote WHM, IP migration wizard | Settings text or confirm pages, not a multi-node product. |

**Operator-visible problem:** the sidebar looks like a finished WHM. After the P0 dedicated pages, the next click often “saves” and changes nothing on the host. That is why the product feels “not normal” even when List Accounts works.

### 7.2 Control vs cPanel

cPanel tenants expect **apps**, not a developer form dump: File Manager with upload/delete, Email with password + webmail, MySQL with users + phpMyAdmin, Zone Editor with delete, SSL status, and an Account section for password and stats.

| cPanel-class app | Control today | Why it feels wrong |
| --- | --- | --- |
| File Manager | Text list + write textarea; FTP/SSH on the same page | Cannot do the file actions the API already supports. Looks like an internal debug tool. |
| Email Accounts | Create/delete + catch-all + aliases | No webmail button, no password change, no quota edit, no lists. Filters/autoresponders absent (and unlabeled). |
| MySQL® Databases | Name + engine + delete | No connection string, no phpMyAdmin, no “user”. Tenants cannot use the database they just created. |
| Zone Editor | Add record only | Cannot remove a bad A record from Control. Director can. |
| SSL/TLS Status | Table + request | Hidden under Websites. No lifecycle. Custom cert not offered (API missing — must be labeled if stubbed). |
| Domains | One attach form | Works, but no docroot, no redirects, no “manage” drawer. |
| Cron | Nested under Backups | Easy to miss; no edit. |
| FTP | Buried in Files | Not an app. |
| Metrics / Softaculous / Redirects / Error pages / IP Blocker | Absent | Fine as **labeled** later-phase stubs; not P0. |
| Account / password / contact | Dashboard three-liner | No place that feels like “my hosting account”. |
| Visual language | Serif, cream, seven links | Reads as a brochure admin, not a hosting app launcher. |

Director Email/SQL/Files **are** closer to normal — so a tenant who is sent to Control after “Login to Kelmor Control” loses manage actions the operator just used. That journey is the sharpest product failure.

---

## 8. Ranked fix list (plan v3)

Do **not** implement this list in the audit PR. Order is product, then honesty, then new backend.

### P0 — Kelmor Control, cPanel-class (six families)

Ship as real apps on the existing APIs. If a manage action has no API, **label the stub**; do not hide the gap.

| Rank | Family | Do this | Reuse | Honest stub only if |
| --- | --- | --- | --- | --- |
| 1 | **Account** | First-class `/` (or `/account`): identity, status, package/usage bars, voluntary password, contact email. Keep forced-change. Drop lab-prefilled login secrets. Multi-account switcher if `account_ids.length > 1`. | `GET /accounts/{id}`, `/usage`, `/password`, `complete-password-change` | 2FA / language until those APIs exist |
| 2 | **Files** | Dedicated File Manager: breadcrumbs, mkdir, upload, rename, delete (confirm), chmod, text edit, protected-path warning. Move FTP/SSH to their own entries or labeled tabs. | `files`, `files/content`, `mkdir`, `DELETE/PATCH files`, `/ftp`, `/ssh-keys`, `/sftp-password` | Extract/compress if Agent pack ops are not wired to the UI |
| 3 | **Email** | Mailboxes table with create, delete, **webmail launch**, IMAP/SMTP copy. Catch-all + aliases. Lists if `mail/lists` is exposed. | mail + `admin-tools` + Director `mail-connection.ts` | Filters, autoresponders, spam — **labeled coming later**. Mailbox password/quota — **labeled** until `PATCH` mailbox exists |
| 4 | **DB** | Database table + create/delete + **reveal/copy credentials** + **Open phpMyAdmin**. Explain one user per engine. | `/databases`, `/databases/credentials`, `/admin-tools` | Extra MySQL users/GRANTs — labeled unsupported (Kelmor model) |
| 5 | **Domains / DNS** | Domains manage + Zone Editor with add **and delete** (and edit via replace). Show DNSSEC/DS read-only or write if `dns.write`. | `/domains`, `/dns/zones…` | Domain forwarding URL map until it is more than `server/settings` |
| 6 | **SSL** | Own nav item (keep `/websites?tab=ssl` redirect). Inventory, request AutoSSL, status, expiry. | `/certificates` | Custom PEM install — labeled until an upload API exists |

**Control chrome (same P0 train):** tool search or obvious app grid, account chip, Kelmor (not WHM/cPanel) visual system aligned with Director’s light professional panel — still Control-branded.

**Acceptance for P0:** a tenant can attach a domain, fix a DNS record, upload a file, create a mailbox and open webmail, create a database and copy credentials / open phpMyAdmin, request TLS, and change their account password — without asking the administrator to use Director.

### P1 — Honesty on Director catalog

| Rank | Fix |
| --- | --- |
| 7 | Label every `settingKey` tool in the UI: “Host preference — not applied to nginx/Postfix/PowerDNS yet.” Or hide from the default sidebar behind “Preferences”. |
| 8 | Wire or remove misleading account tools (Quota Modification, Limit Bandwidth) so Save `PATCH`es the account (or the copy stops claiming Agent enforcement). |
| 9 | Keep dedicated managers as the only unlabeled “this is the tool” destinations (already the P0 Director direction). |

### P2 — Close real API gaps (only if P0 still blocked)

| Rank | Gap | Why it waits |
| --- | --- | --- |
| 10 | `PATCH` mailbox password + quota | Both portals need it for Email P0 manage |
| 11 | Custom certificate install | Only if SSL P0 must accept CA bundles |
| 12 | Record PATCH (optional) | Delete+add is enough if the Zone Editor is clear |
| 13 | php.ini / MultiPHP INI apply | Today a settings stub; needs Agent `ApplyPhpPool` fields |
| 14 | Redirects, error pages, hotlink, IP deny, metrics | New product; labeled stubs on Control after P0 |

### P3 — Depth and polish (do not block P0)

Director UX debt already written down in [`docs/kelmor-audit/consolidated-remaining-gaps.md`](../kelmor-audit/consolidated-remaining-gaps.md): account chip on every scoped tool, DNS edit affordances, credential reveal-by-default, file list growth, webmail URL honesty, cert lifecycle, transfer review. Those remain valid **after** Control P0.

Out of product: billing, copied cPanel/WHM UI, root shell, Windows, K8s.

---

## 9. Suggested Control information architecture (v3, not built)

Keep Kelmor names. Group as apps, not as “hubs that hide cron under backups”:

1. **Account** — overview, password, usage  
2. **Domains** — domains, DNS, redirects (stub)  
3. **Websites** — sites, PHP version (SSL can live here *and* in SSL)  
4. **Files** — manager; FTP; SSH/SFTP  
5. **Email** — mailboxes, aliases, lists, webmail, deliverability (read)  
6. **Databases** — MariaDB/PostgreSQL + phpMyAdmin  
7. **SSL / TLS**  
8. **Backups** — backups; cron as sibling or Account/Advanced  
9. **Advanced** — applications, API tokens, labeled stubs  

Director remains the WHM analog: host, packages, resellers, impersonation, transfers, jobs.

---

## 10. File index

| Path | Role |
| --- | --- |
| `portals/account/src/app.tsx` | Control routes + inline Email/DB/Domains/SSL/Cron/Dash |
| `portals/account/src/control-hubs.ts` | Seven-item nav |
| `portals/account/src/pages/*.tsx` | Files, DNS, websites, backups, password form |
| `portals/server/src/app.tsx` | Director route table |
| `portals/server/src/whm-catalog.ts` | WHM-shaped tool list |
| `portals/server/src/pages/whm-tool-page.tsx` | Generic apply + settings save |
| `portals/server/src/nav-hubs.ts` | Hub vs dedicated path rules |
| `internal/httpserver/api.go` | `/api/v1` registration |
| `internal/httpserver/account_files.go` | File mkdir/delete/rename/chmod |
| `internal/httpserver/director_settings.go` | Preference blob |
| `agent/operations/ops.go` | Typed Agent methods |
| `api/openapi.yaml` | Contract |
| `docs/director-ui.md` | Director operator doc |
| `docs/director-whm-journey-checklist.md` | Director P0 checklist (Director only) |
| `docs/kelmor-mvp-gap.md` | Backend MVP honesty |
| `docs/plans/2026-09-09-001-feat-whm-style-kelmor-director-plan.md` | Director plan (Control OOS) |

---

## 11. What this document is not

- Not a Control implementation.  
- Not a promise that every WHM name will become a live Agent op.  
- Not a license to copy cPanel/WHM.  
- Tiny one-file product bugs (for example lab login prefills) were left for the Control P0 Account slice so this PR stays docs-only.

Re-verify statuses against `git log` if this file is more than a few weeks old; the API surface is the source of truth, not the sidebar label.
