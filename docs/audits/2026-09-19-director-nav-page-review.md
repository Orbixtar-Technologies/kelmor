# Director navigation inventory and page review

**Tree:** `origin/main` at `e72e6d8` (merge of `cursor/director-whm-honesty-d21c`, [PR #38](https://github.com/usmanliaqatdeveloper/kelmor/pull/38)), 19 September 2026.  
**Scope:** Kelmor Director only (`portals/server`). Kelmor Control is out of scope except Director SSO / Login to Control.  
**Follow-up:** host-applied Director tools ship in `cursor/director-should-do-host-c680`. See `director-should-do-progress.md`.

Companion honesty map (families, not every tile): [`2026-09-19-kelmor-portal-vs-hosting-panel.md`](./2026-09-19-kelmor-portal-vs-hosting-panel.md).

---

## 1. Method

Sources of truth, in order:

1. `portals/server/src/whm-catalog.ts` — 191 WHM-shaped features (`id`, `path`, `category`, `layout`, `dedicated`, `settingKey`, `accountAction`).
2. `portals/server/src/nav-hubs.ts` + `operator-nav.ts` + `layout/sidebar.tsx` — operator hrefs. Dedicated managers open their route; other tools open `/section/{hub}?tool={id}` and render `WhmToolBody`. `/tools/{id}` redirects to that href.
3. `portals/server/src/app.tsx` — dedicated React routes.
4. `portals/server/src/pages/whm-tool-page.tsx` `applyFeature` + `catalog-honesty.ts` — what Save actually does after PR #38.
5. Dedicated `pages/*-page.tsx` and `internal/httpserver` (especially `director_settings.go`, `api.go` `modifyAccount`, `worker.go` reconcile).

Every catalog id was dumped through `hrefForFeature` so the **Nav / URL** column is the sidebar/Home destination, not only the catalog `path`.

---

## 2. Status words

| Status | Meaning in this matrix |
| --- | --- |
| **OK** | Named journey is API-backed and usable. Kelmor model limits (one DB user, Postfix lists, recipe “terminal”) are honest. |
| **Partial** | A real manager or API exists, but everyday manage, job follow-through, or WHM-class depth is incomplete. |
| **Stub** | Labeled **Settings (local) / Not applied to host**, or an honest status/pointer with no host mutation. `PATCH /api/v1/server/settings` is a JSON blob; nothing in the Agent or worker reads those keys. |
| **Broken** | Apply/Save does the **wrong** host action, a no-op presented as Apply, or persists fields the worker ignores while looking like a live account-action. |
| **Missing** | No Director route and no API. None of the catalog ids are Missing; extra routes below exist. |

PR #38 labeled settings tiles and HTTPS launches. **Labeled stub ≠ host-applied.** A green sidebar still contains ~74 preference forms.

---

## 3. Summary counts

Inventory: **191 catalog entries** + **6 extra routes** = **197 matrix rows**.

| Status | Catalog | Extra routes | Total |
| --- | ---: | ---: | ---: |
| OK | 32 | 3 | **35** |
| Partial | 61 | 3 | **64** |
| Stub | 93 | 0 | **93** |
| Broken | 5 | 0 | **5** |
| Missing | 0 | 0 | **0** |

Catalog shape:

| Shape | Count |
| --- | ---: |
| Features | 191 |
| `dedicated: true` (opens a first-class route) | 76 |
| `settingKey` / local settings | 74 |
| Sidebar hubs | 14 (`home`, `accounts`, `packages`, `dns`, `email`, `websites`, `files`, `sql`, `ssl`, `backups`, `server`, `security`, `status`, `system`) |

Broken catalog ids: `change-site-ip`, `manage-shell`, `unsuspend-bandwidth`, `file-dir-restore`, `assign-ipv6`.

---

## 4. Best-practice expectations (hosting panels)

These are operator-journey expectations, not a license to copy WHM/cPanel trademarks or chrome.

### 4.1 Account-scoped tools

- Every tenant-affecting tool has an explicit account context (picker, `?account=`, or `/accounts/:id`).
- Switching accounts must not apply the previous account’s response (`RequestSequence` already does this on several managers).
- Cross-links go to the **same** account’s Email, SQL, DNS, SSL, Files, Jobs — not a second hop through a hub tab dump.
- Host-scoped tools (restart, firewall, updates) must not pretend to be per-account.

### 4.2 Jobs feedback

- Any write that returns `operation_id` / 202 should show the job id, poll or link to `/jobs?account=&selected=`, and not say “saved” as if the host already changed.
- Create Account already navigates to Jobs. List suspend, Summary PATCH, Domains, Websites, and Transfers should match that bar.
- Jobs UI should expose **retry and cancel** when the API does (`POST /jobs/{id}/retry` is wired; `POST /jobs/{id}/cancel` is not).

### 4.3 No fake Save

- Buttons that persist `director-settings.json` must stay labeled **Save local preference** / **Not applied to host** (PR #38).
- Account-action **Apply host change** may only run when the worker/agent consumes the field. Today `ip_address` and `shell_class` are stored and ignored — that is fake Apply, even without a settings blob.
- Confirm wizards (`synchronize-dns`, `dns-cleanup`, `convert-addon`, `ip-migration`) must not use the same Save path as a real queue.

### 4.4 HTTPS launches

- Login to Control, webmail, and phpMyAdmin open the **public hostname** over HTTPS (`control-url.ts`, `admin-tool-url.ts`, `portal_urls.go`). Loopback lab ports may stay HTTP.
- Do not launch tenant tools on the Director host, a stale `:8443`, or raw `http://` on a public name. PR #38 is the current contract; keep it.

### 4.5 Privilege and honesty

- Nav visibility is not authorization. API capabilities remain the boundary.
- Kelmor-model limits (one DB user per engine, no mailbox PATCH, no custom PEM, no root shell) must stay labeled — not hidden behind a WHM name.
- In-browser root shell stays **Missing on purpose**.

---

## 5. How navigation works

```text
Sidebar / Home / Find
  → hrefForFeature(feature)
      dedicated && !/tools/*  → feature.path   (e.g. /email?tab=lists)
      else                    → /section/{hub}?tool={id}
  → HubPage renders WhmToolBody  OR  dedicated page from app.tsx
/tools/{id} → same href (redirect)
```

`applyFeature` (generic tools only):

| Pattern | Effect |
| --- | --- |
| `layout: restart` | `POST /server/services/{name}/restart` |
| `unsuspend-bandwidth` | Bulk unsuspend **all** suspended (Broken) |
| `wp-toolkit` | `POST /accounts/{id}/wordpress` |
| `repair-mailbox-perms` | Empty `PATCH /accounts/{id}` (reconcile) |
| `accountAction` password/suspend/terminate/remove | Matching account POSTs |
| `accountAction: impersonate` | Throws — use dedicated Login to Control |
| `accountAction: patch` | PATCH only `ip_address`, `package_id`, `shell_class` |
| `settingKey` / leftover confirm/wizard/form | `PATCH /server/settings` |
| `file-dir-restore` | **Nothing to apply** |

Dedicated managers bypass `applyFeature` and call their own APIs.

---

## 6. Review matrix


### Kelmor Director (1)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `home` → `/` | Kelmor Director | Host overview, Find, vitals, shortcuts into account/job/service tools. | Dedicated `/`. Home vitals, P0 shortcuts, failed-job list, full catalog. Local-settings tiles badged. | **OK** | Measured GET /server, /accounts, /jobs?state=failed. No writes. |

### Account Functions (21)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `create-account` → `/accounts/create` | Account Functions | Reviewed provision of a POSIX tenant with package and ownership. | Dedicated `/accounts/create`. Three-step identity → package → review wizard; POST /accounts; navigates to /jobs?selected=. | **OK** | Provision job is the success path. |
| `change-site-ip` → `/section/accounts?tool=change-site-ip` | Account Functions | Assign dedicated/shared IPv4 and reconcile vhost + DNS. | Catalog tool `/section/accounts?tool=change-site-ip` → `WhmToolBody` (`account-action`). accountAction=patch. PATCH ip_address on the account; worker always publishes publicIPv4() for vhosts/DNS. Field is DB-only. | **Broken** | Looks like a real account-action Apply. |
| `email-all-users` → `/section/accounts?tool=email-all-users` | Account Functions | Send (or queue) mail to every account owner. | Catalog tool `/section/accounts?tool=email-all-users` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Draft subject/body/from → PATCH /server/settings email_all_users. Not mailed. | **Stub** | Copy admits copy-into-relay; still a fake Send-shaped form. |
| `force-password` → `/accounts?task=password` | Account Functions | Rotate owner + Linux password; optionally force Control re-choose. | Dedicated `/accounts?task=password`. accountAction=password. List picker (?task=password) then Account Summary password dialog → POST /accounts/{id}/password. | **OK** | Job id not surfaced after rotate. |
| `limit-bandwidth` → `/section/accounts?tool=limit-bandwidth` | Account Functions | Override monthly transfer for one account (or clearly edit the package). | Catalog tool `/section/accounts?tool=limit-bandwidth` → `WhmToolBody` (`account-action`). accountAction=patch. Account + package picker; `PATCH /accounts/{id}` `package_id`. Agent enforces the **package** `bandwidth_bytes_monthly`. Honesty banner: no per-account override. | **Partial** | Same journey as Upgrade/Downgrade; job link only if `operation_id` is returned. |
| `suspend-account` → `/accounts?task=suspension` | Account Functions | Suspend/unsuspend and lock web/mail/cron/ftp. | Dedicated `/accounts?task=suspension`. accountAction=suspend. List + summary suspend/unsuspend POST. ?task=suspension does not auto-open the suspend UI. | **Partial** | Bulk suspend on list; no operation_id link. |
| `manage-demo-mode` → `/section/accounts?tool=manage-demo-mode` | Account Functions | Mark demo tenants and block destructive owner writes. | Catalog tool `/section/accounts?tool=manage-demo-mode` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Comma-separated usernames stored; no API/RBAC demo lock. | **Stub** | Copy claims destructive owner writes are blocked. |
| `manage-shell` → `/section/accounts?tool=manage-shell` | Account Functions | Set sftp-only / jailed / nologin and apply on the host. | Catalog tool `/section/accounts?tool=manage-shell` → `WhmToolBody` (`account-action`). accountAction=patch. PATCH shell_class persisted; reconcile always uses /usr/sbin/nologin. | **Broken** | sftp-only / jailed never applied. |
| `modify-account` → `/accounts?task=modify` | Account Functions | Change domain, IP, package, reseller, login access with job follow-through. | Dedicated `/accounts?task=modify`. accountAction=patch. Account Summary assignment editor PATCH /accounts/{id} (domain, IP, package, reseller, login_disabled). | **Partial** | UI says “updated” while API queues account.reconcile. IP/shell stored but worker does not apply them. |
| `password-modification` → `/accounts?task=password` | Account Functions | Set owner password without a second forced change. | Dedicated `/accounts?task=password`. accountAction=password. Same password dialog as Force Password Change. | **OK** | No second-change-required flag in this entry’s copy vs force-password. |
| `quota-modification` → `/section/accounts?tool=quota-modification` | Account Functions | Override disk quota for one account (or edit the package). | Catalog tool `/section/accounts?tool=quota-modification` → `WhmToolBody` (`account-action`). accountAction=patch. Account + package picker; `PATCH` `package_id`. Agent `SetFilesystemQuota` uses package `disk_bytes`. Honesty banner: no per-account override. | **Partial** | Duplicate of Change Package under a quota label. |
| `raw-nginx-log` → `/section/accounts?tool=raw-nginx-log` | Account Functions | Download or tail account vhost access/error logs. | Catalog tool `/section/accounts?tool=raw-nginx-log` → `WhmToolBody` (`account-action`). Copies inferred {home}/logs/access.log and error.log. No download API. | **Stub** | Paths may not exist until the vhost writes them. |
| `rearrange-account` → `/section/accounts?tool=rearrange-account` | Account Functions | Move the account home (or honestly say homes are fixed). | Catalog tool `/section/accounts?tool=rearrange-account` → `WhmToolBody` (`account-action`). Status panel shows account.home_path. Homes stay /home/<user>. | **Stub** | Read-only; account picker required for paths. |
| `reset-bandwidth` → `/section/accounts?tool=reset-bandwidth` | Account Functions | Clear a per-account transfer hold / override. | Catalog tool `/section/accounts?tool=reset-bandwidth` → `WhmToolBody` (`status`). Honesty panel: clear override does not exist; use package + unsuspend. | **Stub** | Status layout; no Save. |
| `skeleton-directory` → `/section/accounts?tool=skeleton-directory` | Account Functions | Edit files copied into new homes. | Catalog tool `/section/accounts?tool=skeleton-directory` → `WhmToolBody` (`status`). Explains /etc/skel. No custom skeleton editor. | **Stub** | Honest. |
| `terminate-account` → `/accounts?task=terminate` | Account Functions | Destroy hosted services after typed confirm. | Dedicated `/accounts?task=terminate`. accountAction=terminate. Typed-username terminate on Account Summary → POST /terminate. | **OK** | Reviewed impact copy present. |
| `remove-terminated-account` → `/accounts?view=terminated&task=remove` | Account Functions | Purge terminated identity so user/domain can be reused. | Dedicated `/accounts?view=terminated&task=remove`. accountAction=remove. Terminated view + remove dialog → POST /remove. | **OK** | Frees username/domain. |
| `unsuspend-bandwidth` → `/section/accounts?tool=unsuspend-bandwidth` | Account Functions | Unsuspend only accounts held for transfer quota. | Catalog tool `/section/accounts?tool=unsuspend-bandwidth` → `WhmToolBody` (`confirm`). POST /accounts/bulk/unsuspend for every status=suspended account, not bandwidth holds. | **Broken** | Named tool does the wrong set. |
| `change-package` → `/accounts?task=package` | Account Functions | Upgrade/downgrade package and reconcile limits. | Dedicated `/accounts?task=package`. accountAction=patch. Summary assignment editor PATCH package_id (real package limits). | **Partial** | Same weak job feedback as modify. |
| `login-control` → `/accounts?task=login` | Account Functions | Audited HTTPS Control session as the owner. | Dedicated `/accounts?task=login`. accountAction=impersonate. LoginToControl → POST /impersonate; opens https://<hostname>:2083/#session=. | **OK** | Generic WhmToolPage throws if reached without dedicated page. |
| `cron-jobs` → `/cron` | Account Functions | Account crontab: add, edit, disable, delete, last run. | Dedicated `/cron`. Dedicated /cron: create/delete account cron; QueuedOpNotice. | **Partial** | No edit, disable, or email-on-fail. enabled hardcoded true. |

### Account Information (8)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `list-accounts` → `/accounts` | Account Information | Searchable inventory with lifecycle actions. | Dedicated `/accounts`. Search, sort, paginate, view filters, row tools, Control login, bulk suspend. | **OK** | Suspend success omits Jobs link. |
| `account-summary` → `/accounts?task=summary` | Account Information | Single-account hub: status, usage, services, jobs, lifecycle. | Dedicated `/accounts?task=summary`. Hub for usage, isolation, assignment, lifecycle, recent jobs, deep links. | **Partial** | PATCH/password omit operation_id; ?task=suspension not wired. |
| `list-domains` → `/domains` | Account Information | Inventory addon/sub/parked; add/remove; link DNS/SSL. | Dedicated `/domains`. /domains inventory, add, delete non-primary; type filter all. | **Partial** | Create/delete queue jobs but no QueuedOpNotice. |
| `list-subdomains` → `/domains?view=subdomain` | Account Information | Subdomain inventory + manage. | Dedicated `/domains?view=subdomain`. Same manager with ?view=subdomain. | **Partial** | Filter only; no extra subdomain wizard. |
| `list-parked` → `/domains?view=alias` | Account Information | Parked/alias inventory + optional URL forward. | Dedicated `/domains?view=alias`. Same manager with ?view=alias. | **Partial** | Park is type=alias; no URL forward. |
| `suspended` → `/accounts?view=suspended` | Account Information | Suspended-only list + unsuspend. | Dedicated `/accounts?view=suspended`. /accounts?view=suspended + unsuspend actions. | **OK** | Same job-link gap as list. |
| `over-quota` → `/accounts?view=over-quota` | Account Information | Accounts over disk or transfer vs package. | Dedicated `/accounts?view=over-quota`. /accounts?view=over-quota vs package disk/bandwidth. | **Partial** | Depends on monitor or N usage fetches. |
| `usage` → `/usage` | Account Information | Compare observed disk/transfer (and history if available). | Dedicated `/usage`. /usage compare table (disk, bandwidth, memory, CPU, inodes, processes). | **Partial** | Point-in-time only; /monitor alias. |

### Backup (4)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `backup-config` → `/section/backups?tool=backup-config` | Backup | Host backup destination, schedule, retention actually used by the worker. | Catalog tool `/section/backups?tool=backup-config` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Destination/retention/encrypt preference only. Real backups use POST /accounts/{id}/backups. | **Stub** | HPM1 encrypt checkbox unused. |
| `backup-restoration` → `/transfers` | Backup | Restore an HPM1 archive with scope review + job. | Dedicated `/transfers`. Lands on /transfers; restore also on account services. | **Partial** | Restore not a first-class wizard on this tile. |
| `backup-user-selection` → `/section/backups?tool=backup-user-selection` | Backup | Include/exclude accounts from scheduled backups. | Catalog tool `/section/backups?tool=backup-user-selection` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Username list preference. Scheduler does not read it. | **Stub** |  |
| `file-dir-restore` → `/section/backups?tool=file-dir-restore` | Backup | Restore one home path from backup without full account restore. | Catalog tool `/section/backups?tool=file-dir-restore` → `WhmToolBody` (`account-action`). account-action with no apply handler → “Nothing to apply.” Real restore is POST /accounts/{id}/restores on services. | **Broken** | Save is inert. |

### Transfers (3)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `transfers` → `/transfers` | Transfers | Export/import/copy accounts with collision review and jobs. | Dedicated `/transfers`. Export, native import, cPanel-tree import, account copy; reviewed payloads. | **Partial** | Import/copy omit Jobs navigation. |
| `copy-account` → `/transfers` | Transfers | Copy/import an account from another source. | Dedicated `/transfers`. /transfers copy journey. | **Partial** | Job follow-through weak. |
| `review-transfers` → `/jobs?q=transfer` | Transfers | Follow transfer/restore jobs. | Dedicated `/jobs?q=transfer`. /jobs?q=transfer. | **Partial** | Search filter only. |

### Clusters (3)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `configuration-cluster` → `/section/server?tool=configuration-cluster` | Clusters | Share packages/features with peer Directors — or omit the product. | Catalog tool `/section/server?tool=configuration-cluster` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Peer URL textarea. No cluster product. | **Stub** |  |
| `dns-cluster` → `/section/server?tool=dns-cluster` | Clusters | Separate authoritative NS cluster — or omit. | Catalog tool `/section/server?tool=dns-cluster` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Nameserver list preference. Live DNS is PowerDNS on this node + /dns. | **Stub** |  |
| `remote-access-key` → `/section/server?tool=remote-access-key` | Clusters | Issue a Director-to-Director token. | Catalog tool `/section/server?tool=remote-access-key` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Key name saved; secrets stripped. Real tokens are account API tokens. | **Stub** |  |

### DNS Functions (13)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `dns` → `/dns` | DNS Functions | Edit zone records and DNSSEC for an account. | Dedicated `/dns`. Zones, add/edit/duplicate/delete records, DNSSEC, job watch, account scope. | **OK** | Edit = delete+add; no zone templates. |
| `add-dns-zone` → `/domains` | DNS Functions | Create a managed zone. | Dedicated `/domains`. Redirects to /domains add-domain (creates managed zone). | **Partial** | Not a standalone zone wizard. |
| `add-hostname-a` → `/section/dns?tool=add-hostname-a` | DNS Functions | Publish A for the panel hostname. | Catalog tool `/section/dns?tool=add-hostname-a` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). IPv4 preference. Does not write the host DNS zone. | **Stub** |  |
| `delete-dns-zone` → `/domains` | DNS Functions | Remove a non-primary zone after confirm. | Dedicated `/domains`. /domains delete non-primary. | **Partial** | Primary zone cannot be deleted here. |
| `edit-zone-templates` → `/section/dns?tool=edit-zone-templates` | DNS Functions | Default TTL/SOA/records for new zones. | Catalog tool `/section/dns?tool=edit-zone-templates` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). TTL/SOA/extra records preference. New zones use code defaults. | **Stub** |  |
| `email-routing-dns` → `/email?tab=domains` | DNS Functions | MX / local vs remote mail routing. | Dedicated `/email?tab=domains`. /email?tab=domains catch-all PATCH. | **Partial** | Not MX/routing wizard; no AutoSPF. |
| `dkim-spf-global` → `/section/dns?tool=dkim-spf-global` | DNS Functions | Require and provision SPF/DKIM/DMARC on every mail domain. | Catalog tool `/section/dns?tool=dkim-spf-global` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Require SPF/DKIM/DMARC checkboxes unused by provision. | **Stub** | Deliverability is per-account DNS. |
| `dns-cleanup` → `/section/dns?tool=dns-cleanup` | DNS Functions | Scan and delete leftover zones for terminated accounts. | Catalog tool `/section/dns?tool=dns-cleanup` → `WhmToolBody` (`confirm`). Confirm Save writes settings key dns-cleanup. No leftover-zone scan. | **Stub** |  |
| `set-zone-ttl` → `/section/dns?tool=set-zone-ttl` | DNS Functions | Default TTL applied to new/updated records. | Catalog tool `/section/dns?tool=set-zone-ttl` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). TTL preference unused by record writes. | **Stub** |  |
| `domain-forwarding` → `/section/dns?tool=domain-forwarding` | DNS Functions | HTTP(S) forward a parked domain to a URL. | Catalog tool `/section/dns?tool=domain-forwarding` → `WhmToolBody` (`account-action`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). account + source + target URL → domain_forwards settings. nginx/domains API unused. | **Stub** |  |
| `synchronize-dns` → `/section/dns?tool=synchronize-dns` | DNS Functions | Re-publish every managed zone to PowerDNS. | Catalog tool `/section/dns?tool=synchronize-dns` → `WhmToolBody` (`confirm`). Confirm writes settings. Real sync is per-zone from /dns. | **Stub** | Copy says re-publish every zone. |
| `park-domain` → `/domains?view=alias` | DNS Functions | Add an alias/parked domain. | Dedicated `/domains?view=alias`. /domains?view=alias add alias. | **Partial** | No forwarding URL. |
| `edit-dns-zone` → `/dns` | DNS Functions | Open the zone editor. | Dedicated `/dns`. Opens /dns. | **OK** | Same as DNS Zone Manager. |

### Email (15)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `email` → `/email` | Email | Mailboxes, aliases, lists, routing; password/quota manage. | Dedicated `/email`. Tabs: domains, mailboxes, aliases, lists; webmail launch; job watch. | **Partial** | Mailbox password/quota stubbed (no PATCH API). |
| `deliverability` → `/deliverability` | Email | Inspect and repair SPF/DKIM/DMARC. | Dedicated `/deliverability`. Read-only SPF/DKIM/DMARC from DNS TXT. | **Partial** | No Fix/provision buttons. |
| `webmail` → `/webmail` | Email | HTTPS webmail launch + client settings. | Dedicated `/webmail`. IMAP/SMTP copy + HTTPS Open webmail via admin-tools. | **Partial** | No per-mailbox SSO; viewAll not in URL. |
| `filter-country` → `/section/email?tool=filter-country` | Email | Reject/greylist by country in the mail stack. | Catalog tool `/section/email?tool=filter-country` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Country filter preference. rspamd not configured from this key. | **Stub** |  |
| `filter-domain` → `/section/email?tool=filter-domain` | Email | Allow/deny sending domains in the mail stack. | Catalog tool `/section/email?tool=filter-domain` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Allow/deny domain lists unused. | **Stub** |  |
| `greylisting` → `/section/email?tool=greylisting` | Email | Defer unknown senders in Postfix/rspamd. | Catalog tool `/section/email?tool=greylisting` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Enable/delay preference unused. | **Stub** |  |
| `mail-queue` → `/section/email?tool=mail-queue` | Email | Inspect/flush/hold/delete Postfix queue entries. | Catalog tool `/section/email?tool=mail-queue` → `WhmToolBody` (`status`). Same Postfix recipes + Jobs/Deliverability links. | **Partial** | No per-message hold/delete UI. |
| `mail-delivery-reports` → `/jobs?q=mail` | Email | Per-recipient delivery history. | Dedicated `/jobs?q=mail`. /jobs?q=mail filter. | **Partial** | Not a Postfix delivery-report product; Jobs search only. |
| `spamd-startup` → `/section/email?tool=spamd-startup` | Email | Spam scoring actually applied by rspamd. | Catalog tool `/section/email?tool=spamd-startup` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). rspamd thresholds unused. | **Stub** |  |
| `repair-mailbox-perms` → `/section/email?tool=repair-mailbox-perms` | Email | Fix Maildir UID/GID via a dedicated mail.maps job. | Catalog tool `/section/email?tool=repair-mailbox-perms` → `WhmToolBody` (`account-action`). Empty PATCH /accounts/{id} queues reconcile → applyMailStack. | **Partial** | Works only as full account reconcile; no dedicated mail.maps job copy. |
| `mailman` → `/email?tab=lists` | Email | Mailing lists (Kelmor: Postfix multi-member aliases). | Dedicated `/email?tab=lists`. /email?tab=lists Postfix multi-member aliases. | **Partial** | Not GNU Mailman; StatusPanel also links here. |
| `track-delivery` → `/jobs?q=mail` | Email | Find delivery attempts for a recipient. | Dedicated `/jobs?q=mail`. /jobs?q=mail. | **Partial** | Duplicate of mail-delivery-reports. |
| `view-relayers` → `/section/email?tool=view-relayers` | Email | Accounts allowed to relay. | Catalog tool `/section/email?tool=view-relayers` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Relay usernames unused. | **Stub** |  |
| `mail-troubleshooter` → `/section/email?tool=mail-troubleshooter` | Email | Guided deferred-mail checklist + live queue tools. | Catalog tool `/section/email?tool=mail-troubleshooter` → `WhmToolBody` (`status`). Checklist + Postfix recipes (queue/flush/status). | **Partial** | Recipes are API-backed; “fix” is manual. |
| `configure-email-client` → `/webmail` | Email | IMAP/submission settings tenants can copy. | Dedicated `/webmail`. Opens /webmail settings. | **Partial** | No mobile profiles. |

### IP Functions (8)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `add-ip` → `/section/server?tool=add-ip` | IP Functions | Add a host address to the assignable pool and interface. | Catalog tool `/section/server?tool=add-ip` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). ip_pool preference. No interface/address apply. | **Stub** |  |
| `assign-ipv6` → `/section/server?tool=assign-ipv6` | IP Functions | Assign IPv6 to an account and publish AAAA/vhost. | Catalog tool `/section/server?tool=assign-ipv6` → `WhmToolBody` (`account-action`). accountAction=patch. Same PATCH ip_address path; no IPv6 apply in worker. | **Broken** | Can overwrite IPv4 field with an IPv6 string. |
| `remote-service-ips` → `/section/server?tool=remote-service-ips` | IP Functions | Configure remote MySQL/DNS/mail service addresses if offered. | Catalog tool `/section/server?tool=remote-service-ips` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). mysql/dns/mail IP prefs unused. | **Stub** |  |
| `ip-migration` → `/section/server?tool=ip-migration` | IP Functions | Move many accounts from one IP to another. | Catalog tool `/section/server?tool=ip-migration` → `WhmToolBody` (`wizard`). Wizard Save writes settings (feature id). No account PATCH loop. | **Stub** |  |
| `ipv6-ranges` → `/section/server?tool=ipv6-ranges` | IP Functions | Define assignable IPv6 CIDRs. | Catalog tool `/section/server?tool=ipv6-ranges` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). CIDR textarea unused. | **Stub** |  |
| `rebuild-ip-pool` → `/section/server?tool=rebuild-ip-pool` | IP Functions | Rebuild the assignable IPv4 pool from live interfaces. | Catalog tool `/section/server?tool=rebuild-ip-pool` → `WhmToolBody` (`confirm`). Confirm writes settings. No pool rebuild. | **Stub** |  |
| `show-ip-usage` → `/accounts` | IP Functions | Which accounts use which IPs. | Dedicated `/accounts`. List Accounts table (IP column via account record). | **Partial** | No dedicated IP-usage report. |
| `delete-ip` → `/section/server?tool=delete-ip` | IP Functions | Remove an address from the live pool. | Catalog tool `/section/server?tool=delete-ip` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Removal list unused. | **Stub** |  |

### Networking Setup (3)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `change-hostname` → `/section/server?tool=change-hostname` | Networking Setup | Set host FQDN used by mail, chrome, and certs. | Catalog tool `/section/server?tool=change-hostname` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). hostname key unused. Chrome uses GET /server hostname. | **Stub** |  |
| `resolver-config` → `/section/server?tool=resolver-config` | Networking Setup | Write recursive resolvers the host uses. | Catalog tool `/section/server?tool=resolver-config` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Resolver IPs unused (/etc/resolv.conf not written). | **Stub** |  |
| `nameserver-ips` → `/section/server?tool=nameserver-ips` | Networking Setup | Public ns1/ns2 addresses used in templates. | Catalog tool `/section/server?tool=nameserver-ips` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). ns1/ns2 IPv4 unused. | **Stub** |  |

### Packages (4)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `packages` → `/packages` | Packages | CRUD resource packages that provision enforces. | Dedicated `/packages`. Create/edit/safe-delete packages; assignment counts. | **OK** | reseller_id is a raw text field. |
| `add-package` → `/packages` | Packages | Define a new limit set. | Dedicated `/packages`. Same /packages create form. | **OK** | No clone. |
| `delete-package` → `/packages` | Packages | Delete unused packages. | Dedicated `/packages`. Name-confirmed delete; blocked while assigned. | **OK** | Honest guard. |
| `features` → `/features` | Packages | Create/edit feature sets and assign to packages. | Dedicated `/features`. Read-only GET /feature-sets + packages using each set. | **Stub** | No feature-set write API. |

### Resellers (7)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `resellers` → `/resellers` | Resellers | Reseller identity, privileges, nameservers, owned accounts. | Dedicated `/resellers`. List/create/edit identity, status, nameservers, privilege mask. | **Partial** | No delete, usage, IP delegation, or account assignment UI. |
| `change-ownership` → `/accounts?task=modify` | Resellers | Move one account to a reseller or to root. | Dedicated `/accounts?task=modify`. accountAction=patch. Summary assignment editor reseller_id PATCH. | **Partial** | Single account only; job feedback weak. |
| `change-ownership-bulk` → `/section/packages?tool=change-ownership-bulk` | Resellers | Move many accounts in one reviewed job. | Catalog tool `/section/packages?tool=change-ownership-bulk` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Usernames + reseller_id plan only. | **Stub** |  |
| `email-resellers` → `/section/packages?tool=email-resellers` | Resellers | Mail every reseller contact. | Catalog tool `/section/packages?tool=email-resellers` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Draft only. | **Stub** |  |
| `reseller-ip-delegation` → `/section/packages?tool=reseller-ip-delegation` | Resellers | Reserve IPs a reseller may assign. | Catalog tool `/section/packages?tool=reseller-ip-delegation` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). IP list unused. | **Stub** |  |
| `reseller-shared-ip` → `/section/packages?tool=reseller-shared-ip` | Resellers | Set the reseller shared IP. | Catalog tool `/section/packages?tool=reseller-shared-ip` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Shared IP unused. | **Stub** |  |
| `reseller-usage` → `/resellers` | Resellers | Reseller usage and account status. | Dedicated `/resellers`. Same /resellers list; no usage meters. | **Stub** | WHM usage/manage-status not implemented. |

### Restart Services (8)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `restart-http` → `/section/status?tool=restart-http` | Restart Services | Reload/restart nginx after confirm. | Catalog tool `/section/status?tool=restart-http` → `WhmToolBody` (`restart`). accountAction=service-restart. POST /server/services/nginx/restart. | **OK** | Typed agent ControlService. |
| `restart-dns` → `/section/status?tool=restart-dns` | Restart Services | Restart PowerDNS. | Catalog tool `/section/status?tool=restart-dns` → `WhmToolBody` (`restart`). accountAction=service-restart. POST /server/services/pdns/restart. | **OK** |  |
| `restart-imap` → `/section/status?tool=restart-imap` | Restart Services | Restart Dovecot. | Catalog tool `/section/status?tool=restart-imap` → `WhmToolBody` (`restart`). accountAction=service-restart. POST /server/services/dovecot/restart. | **OK** |  |
| `restart-mail` → `/section/status?tool=restart-mail` | Restart Services | Restart Postfix. | Catalog tool `/section/status?tool=restart-mail` → `WhmToolBody` (`restart`). accountAction=service-restart. POST /server/services/postfix/restart. | **OK** |  |
| `restart-php` → `/section/status?tool=restart-php` | Restart Services | Restart php-fpm. | Catalog tool `/section/status?tool=restart-php` → `WhmToolBody` (`restart`). accountAction=service-restart. POST /server/services/php-fpm/restart. | **OK** |  |
| `restart-mysql` → `/section/status?tool=restart-mysql` | Restart Services | Restart MariaDB. | Catalog tool `/section/status?tool=restart-mysql` → `WhmToolBody` (`restart`). accountAction=service-restart. POST /server/services/mariadb/restart. | **OK** |  |
| `restart-pgsql` → `/section/status?tool=restart-pgsql` | Restart Services | Restart PostgreSQL. | Catalog tool `/section/status?tool=restart-pgsql` → `WhmToolBody` (`restart`). accountAction=service-restart. POST /server/services/postgresql/restart. | **OK** |  |
| `restart-ssh` → `/section/status?tool=restart-ssh` | Restart Services | Restart sshd with session warning. | Catalog tool `/section/status?tool=restart-ssh` → `WhmToolBody` (`restart`). accountAction=service-restart. POST /server/services/sshd/restart. Warns sessions drop. | **OK** |  |

### Security Center (14)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `mod-userdir` → `/section/security?tool=mod-userdir` | Security Center | Disable ~user URL access in the web stack. | Catalog tool `/section/security?tool=mod-userdir` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Policy checkbox unused. nginx vhosts already hostname-only. | **Stub** |  |
| `compiler-access` → `/section/security?tool=compiler-access` | Security Center | Allow/deny gcc/make for tenants. | Catalog tool `/section/security?tool=compiler-access` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Allow compilers unused. | **Stub** |  |
| `security-policies` → `/section/security?tool=security-policies` | Security Center | Idle timeout, password age, admin 2FA — enforced. | Catalog tool `/section/security?tool=security-policies` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Password age / idle / 2FA prefs unused by auth. | **Stub** |  |
| `cphulk` → `/section/security?tool=cphulk` | Security Center | Lock out brute-force sources. | Catalog tool `/section/security?tool=cphulk` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). brute_force thresholds unused. No lockout worker. | **Stub** |  |
| `host-access` → `/section/security?tool=host-access` | Security Center | Allow/deny Director by CIDR (live firewall). | Catalog tool `/section/security?tool=host-access` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). CIDR allow/deny unused. Real apply is /security firewall. | **Stub** | Misleading vs Security page. |
| `external-auth` → `/section/security?tool=external-auth` | Security Center | OIDC/SAML for operators. | Catalog tool `/section/security?tool=external-auth` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). OIDC/SAML issuer unused. Login is local username/password. | **Stub** |  |
| `password-strength` → `/section/security?tool=password-strength` | Security Center | Enforced complexity for account/mailbox passwords. | Catalog tool `/section/security?tool=password-strength` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Complexity prefs unused by password APIs. | **Stub** |  |
| `security-advisor` → `/security` | Security Center | Hardening checklist with links to fixes. | Dedicated `/security`. Same /security page as firewall/reboot. | **Partial** | No scored hardening checklist. |
| `smtp-restrictions` → `/section/security?tool=smtp-restrictions` | Security Center | Block tenant outbound SMTP except Postfix. | Catalog tool `/section/security?tool=smtp-restrictions` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Outbound SMTP restrict unused. | **Stub** |  |
| `two-factor` → `/section/security?tool=two-factor` | Security Center | TOTP enroll + require for admins. | Catalog tool `/section/security?tool=two-factor` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Require TOTP unused. No TOTP enroll API. | **Stub** |  |
| `security` → `/security` | Security Center | Review/apply host firewall; typed reboot. | Dedicated `/security`. Firewall GET + typed APPLY; typed REBOOT; audit link. | **Partial** | Rules not editable; host-access catalog tile is a stub. |
| `audit` → `/audit` | Security Center | Searchable privileged action trail. | Dedicated `/audit`. Client filter of GET /audit-events (default 200). | **Partial** | API q/action/since unused; no export. |
| `wheel-group` → `/section/security?tool=wheel-group` | Security Center | Manage sudoers. | Catalog tool `/section/security?tool=wheel-group` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). sudo members unused. | **Stub** |  |
| `ssh-keys-root` → `/section/security?tool=ssh-keys-root` | Security Center | Manage root authorized_keys on the host. | Catalog tool `/section/security?tool=ssh-keys-root` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Authorized keys textarea unused. Tenant keys are /accounts/{id}/ssh-keys. | **Stub** |  |

### Server Configuration (10)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `basic-setup` → `/section/server?tool=basic-setup` | Server Configuration | Contact, NS, NIC used by provision. | Catalog tool `/section/server?tool=basic-setup` → `WhmToolBody` (`wizard`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Contact/NS/ethernet wizard unused. | **Stub** |  |
| `change-root-password` → `/section/server?tool=change-root-password` | Server Configuration | Set Ubuntu root password once; never store. | Catalog tool `/section/server?tool=change-root-password` → `WhmToolBody` (`status`). HostPasswordForm → POST /server/root-password. Not stored. | **OK** |  |
| `server-profile` → `/section/server?tool=server-profile` | Server Configuration | Hide unused services for mail/DNS-only roles. | Catalog tool `/section/server?tool=server-profile` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). standard/mail/dns profile unused; catalog not filtered. | **Stub** |  |
| `tweak-settings` → `/section/server?tool=tweak-settings` | Server Configuration | Host defaults that provision and daemons read. | Catalog tool `/section/server?tool=tweak-settings` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Host-wide mail/domain/PHP defaults unused. | **Stub** | Classic WHM page; still a blob. |
| `update-preferences` → `/updates` | Server Configuration | Panel release channel and auto-install. | Dedicated `/updates`. /updates channel + automatic toggle PATCH /server/updates/settings. | **OK** | Panel self-update, not OS packages. |
| `configure-cron` → `/section/server?tool=configure-cron` | Server Configuration | Schedules the worker actually uses. | Catalog tool `/section/server?tool=configure-cron` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Host cron schedules unused. Worker intervals are code. | **Stub** |  |
| `initial-quota` → `/section/server?tool=initial-quota` | Server Configuration | Confirm usrquota / Agent disk enforcement. | Catalog tool `/section/server?tool=initial-quota` → `WhmToolBody` (`status`). Explains package quota enforcement. | **Stub** | Honest status. |
| `link-nodes` → `/section/server?tool=link-nodes` | Server Configuration | Attach extra compute/DNS nodes — or omit. | Catalog tool `/section/server?tool=link-nodes` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Node URLs unused. | **Stub** |  |
| `terminal` → `/section/server?tool=terminal` | Server Configuration | Audited host recipes or a real guarded console — never a freeform hidden shell. | Catalog tool `/section/server?tool=terminal` → `WhmToolBody` (`status`). HostConsolePanel: allow-listed recipes (nginx test/reload, postfix queue/flush/status). | **Partial** | Not a root shell — copy is honest; WHM Terminal is not this. |
| `server-time` → `/section/server?tool=server-time` | Server Configuration | Timezone for cron, backups, audit. | Catalog tool `/section/server?tool=server-time` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Timezone unused. Display uses browser/UTC. | **Stub** |  |

### Server Contacts (2)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `contact-manager` → `/section/server?tool=contact-manager` | Server Contacts | Who gets disk/service/update alerts. | Catalog tool `/section/server?tool=contact-manager` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Alert recipients unused. No notifier. | **Stub** |  |
| `system-mail-prefs` → `/section/server?tool=system-mail-prefs` | Server Contacts | From-address and relay for Director mail. | Catalog tool `/section/server?tool=system-mail-prefs` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). From/relay unused. | **Stub** |  |

### Server Status (5)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `services` → `/status` | Server Status | Service health + safe/disruptive controls + job result. | Dedicated `/status`. /status vitals + managed service start/stop/reload/restart. | **Partial** | Job id not shown; process table is control-plane only. |
| `server-information` → `/status` | Server Status | Live hostname, load, memory, disk, uptime. | Dedicated `/status`. Same /status vitals. | **Partial** | No hardware inventory beyond probe. |
| `task-queue` → `/jobs` | Server Status | Background jobs with retry and cancel. | Dedicated `/jobs`. /jobs list, filter, retry; cancel API unused in UI. | **Partial** | No poll while running. |
| `daily-process-log` → `/processes` | Server Status | Historical process snapshots. | Dedicated `/processes`. /processes live snapshot (not a daily log). | **Partial** | Name oversells; one-shot /proc. |
| `apache-status` → `/status` | Server Status | HTTP server workers/connections. | Dedicated `/status`. Opens /status. | **Partial** | nginx workers not a dedicated scoreboard. |

### Service Configuration (11)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `service-manager` → `/status` | Service Configuration | Enable monitoring/control per daemon. | Dedicated `/status`. Same /status service table. | **Partial** | No enable/disable monitor policy. |
| `nameserver-selection` → `/section/server?tool=nameserver-selection` | Service Configuration | Choose/disable the nameserver stack. | Catalog tool `/section/server?tool=nameserver-selection` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). pdns/disabled preference. PowerDNS is always the stack. | **Stub** |  |
| `mailserver-config` → `/section/server?tool=mailserver-config` | Service Configuration | IMAP/submission ports applied to Dovecot/Postfix. | Catalog tool `/section/server?tool=mailserver-config` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). IMAP/submission ports unused (993/587 in copy only). | **Stub** |  |
| `ftp-server-config` → `/section/server?tool=ftp-server-config` | Service Configuration | vsftpd PASV/banner applied. | Catalog tool `/section/server?tool=ftp-server-config` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). PASV range unused. | **Stub** |  |
| `ftp-server-selection` → `/section/server?tool=ftp-server-selection` | Service Configuration | Enable/disable virtual FTP. | Catalog tool `/section/server?tool=ftp-server-selection` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). vsftpd enable unused. | **Stub** |  |
| `exim-config` → `/section/server?tool=exim-config` | Service Configuration | Postfix/rspamd policy applied (do not pretend Exim). | Catalog tool `/section/server?tool=exim-config` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Postfix/rspamd policy blob. Not Exim; not applied. | **Stub** |  |
| `directory-index` → `/section/server?tool=directory-index` | Service Configuration | Index filenames nginx uses. | Catalog tool `/section/server?tool=directory-index` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). index filenames unused by nginx templates. | **Stub** |  |
| `log-rotation` → `/section/server?tool=log-rotation` | Service Configuration | Retention logrotate actually uses. | Catalog tool `/section/server?tool=log-rotation` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Retain days unused. | **Stub** |  |
| `service-ssl` → `/ssl?task=service` | Service Configuration | Host/service certificates (panel, mail, SQL). | Dedicated `/ssl?task=service`. /ssl?task=service informational copy. | **Stub** | No host-cert API. |
| `apache-configuration` → `/section/server?tool=apache-configuration` | Service Configuration | nginx defaults written into new vhosts. | Catalog tool `/section/server?tool=apache-configuration` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). nginx client_max_body_size / keepalive unused. | **Stub** |  |
| `statistics-software` → `/section/server?tool=statistics-software` | Service Configuration | Toggle usage collection the worker honors. | Catalog tool `/section/server?tool=statistics-software` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Collect bandwidth/disk checkboxes unused. Worker collects unconditionally. | **Stub** |  |

### Software (11)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `websites` → `/websites` | Software | Create sites, PHP version, docroot, enable/disable. | Dedicated `/websites`. MultiPHP inventory + create + PHP version POST. | **Partial** | No QueuedOpNotice; docroot fixed on create; no disable. |
| `multiphp-ini` → `/section/websites?tool=multiphp-ini` | Software | php.ini applied to new/existing pools. | Catalog tool `/section/websites?tool=multiphp-ini` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). php.ini defaults unused. Pools use Agent ApplyPhpPool code defaults. | **Stub** |  |
| `nginx-manager` → `/section/websites?tool=nginx-manager` | Software | Default vhost HTTPS/cache policy applied. | Catalog tool `/section/websites?tool=nginx-manager` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). HTTPS redirect/gzip unused. | **Stub** |  |
| `wp-toolkit` → `/section/websites?tool=wp-toolkit` | Software | Install WordPress into a site with a job. | Catalog tool `/section/websites?tool=wp-toolkit` → `WhmToolBody` (`account-action`). Account + website picker → POST /accounts/{id}/wordpress. | **OK** | Needs websites.write + applications.write. |
| `easyapache` → `/section/websites?tool=easyapache` | Software | Install php-fpm versions the MultiPHP manager can assign. | Catalog tool `/section/websites?tool=easyapache` → `WhmToolBody` (`status`). PHPRuntimePanel GET/POST /server/runtimes (8.3/8.4/8.5). | **OK** | Installs php-fpm versions; assignment is MultiPHP Manager. |
| `module-installers` → `/section/websites?tool=module-installers` | Software | Install PECL/PEAR/Perl/Ruby modules on the host — or omit. | Catalog tool `/section/websites?tool=module-installers` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). PECL/PEAR/Perl/Ruby lists unused. | **Stub** |  |
| `perl-modules` → `/section/websites?tool=perl-modules` | Software | Queue a real apt/cpan install job — or omit. | Catalog tool `/section/websites?tool=perl-modules` → `WhmToolBody` (`form`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Module name saved only. | **Stub** |  |
| `php-pear` → `/section/websites?tool=php-pear` | Software | Queue a real PEAR install — or omit. | Catalog tool `/section/websites?tool=php-pear` → `WhmToolBody` (`form`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Package name saved only. | **Stub** |  |
| `php-pecl` → `/section/websites?tool=php-pecl` | Software | Queue a real PECL install — or omit. | Catalog tool `/section/websites?tool=php-pecl` → `WhmToolBody` (`form`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Extension name saved only. | **Stub** |  |
| `ruby-gems` → `/section/websites?tool=ruby-gems` | Software | Queue a real gem install — or omit. | Catalog tool `/section/websites?tool=ruby-gems` → `WhmToolBody` (`form`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Gem name saved only. | **Stub** |  |
| `rebuild-rpm` → `/section/websites?tool=rebuild-rpm` | Software | apt update / package-index refresh job (Ubuntu). | Catalog tool `/section/websites?tool=rebuild-rpm` → `WhmToolBody` (`confirm`). Confirm writes settings. No apt update job. Ubuntu/apt honesty in copy. | **Stub** |  |

### SQL Services (8)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `sql` → `/sql` | SQL Services | Create/delete DBs, show credentials, launch HTTPS phpMyAdmin. | Dedicated `/sql`. Create/delete DB, credentials, HTTPS phpMyAdmin, job watch. | **Partial** | One user/engine; credentials fetch hardcoded mariadb. |
| `mysql-root-password` → `/section/sql?tool=mysql-root-password` | SQL Services | Rotate MariaDB root; never store. | Catalog tool `/section/sql?tool=mysql-root-password` → `WhmToolBody` (`status`). POST /server/database-root-password; not stored. | **OK** |  |
| `db-user-password` → `/sql` | SQL Services | Rotate the account DB user password. | Dedicated `/sql`. Opens Database Manager; no rotate-user API. | **Stub** | Honest “open manager”; password is auto-provisioned file. |
| `additional-mysql-hosts` → `/section/sql?tool=additional-mysql-hosts` | SQL Services | Remote hosts allowed to reach MariaDB. | Catalog tool `/section/sql?tool=additional-mysql-hosts` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Allowed hosts unused. Remote MySQL not a product. | **Stub** |  |
| `postgres-config` → `/section/sql?tool=postgres-config` | SQL Services | Tenant PG listen/auth if offered. | Catalog tool `/section/sql?tool=postgres-config` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Listen/auth unused (control-plane PG). | **Stub** |  |
| `show-mysql-processes` → `/sql` | SQL Services | Show/kill MariaDB threads. | Dedicated `/sql`. Opens /sql; no processlist API. | **Stub** | Copy says process lists stay on the SQL host. |
| `mysql-upgrade` → `/section/sql?tool=mysql-upgrade` | SQL Services | Plan + typed Agent MariaDB major upgrade. | Catalog tool `/section/sql?tool=mysql-upgrade` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Target version preference. Copy still says the agent applies it; Save does not. | **Stub** | Labeled Settings (local); description remains overstated. |
| `phpmyadmin` → `/section/sql?tool=phpmyadmin` | SQL Services | Publish phpmyadmin.<domain> and open HTTPS. | Catalog tool `/section/sql?tool=phpmyadmin` → `WhmToolBody` (`status`). HostAppsPanel kind=sql: enable phpMyAdmin + HTTPS open. | **OK** | Needs account + apps enable. |

### System Health (3)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `processes` → `/processes` | System Health | Inspect /proc and signal leftover tenant processes. | Dedicated `/processes`. GET /server/processes + TERM/KILL protected-pid guard. | **Partial** | Not account-scoped; no refresh timer. |
| `background-killer` → `/section/status?tool=background-killer` | System Health | Auto-kill listed binaries under tenant UIDs. | Catalog tool `/section/status?tool=background-killer` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Binary names unused. | **Stub** |  |
| `disk-usage` → `/usage` | System Health | Host + account disk observations. | Dedicated `/usage`. Same /usage page. | **Partial** | Host disk also on Home/Status. |

### System Reboot (2)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `graceful-reboot` → `/security` | System Reboot | Drain then typed reboot. | Dedicated `/security`. /security typed REBOOT (agent always). | **Partial** | No drain/maintenance window; same path as forceful. |
| `forceful-reboot` → `/security` | System Reboot | Immediate typed reboot if distinct; otherwise one honest path. | Dedicated `/security`. Same typed reboot. | **Partial** | Kelmor does not expose a harder reboot. |

### Files (2)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `file-manager` → `/files` | Files | Browse/upload/mkdir/rename/chmod/delete/edit in the home tree. | Dedicated `/files`. Browse, upload, mkdir, rename, chmod, delete, text edit, protected-path confirm. | **Partial** | No archive/extract, bulk, or ownership. |
| `ftp` → `/ftp` | Files | Virtual FTP users chrooted to account content. | Dedicated `/ftp`. Virtual FTP CRUD + job watch. | **Partial** | No quota/session limits. |

### SSL/TLS (6)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `ssl` → `/ssl` | SSL/TLS | Inventory, AutoSSL, status, optional custom PEM. | Dedicated `/ssl`. Inventory, AutoSSL request, retry, expiry labels, job watch. | **Partial** | Custom PEM labeled missing; task=status ≈ inventory. |
| `generate-csr` → `/ssl?task=request` | SSL/TLS | CSR or AutoSSL request for a hostname. | Dedicated `/ssl?task=request`. /ssl?task=request AutoSSL hostname POST. | **Partial** | No CSR/PEM download; ACME only. |
| `manage-autossl` → `/ssl?task=autossl` | SSL/TLS | Request/renew ACME certs; show policy. | Dedicated `/ssl?task=autossl`. /ssl?task=autossl request form. | **Partial** | No AutoSSL provider policy UI. |
| `install-ssl` → `/ssl?task=request` | SSL/TLS | Install a cert on a hostname (ACME or PEM). | Dedicated `/ssl?task=request`. /ssl?task=request. | **Partial** | Same as generate-csr. |
| `ssl-storage` → `/ssl?task=inventory` | SSL/TLS | Issued certificate inventory. | Dedicated `/ssl?task=inventory`. /ssl?task=inventory. | **Partial** | Inventory only. |
| `ssl-tls-status` → `/ssl?task=status` | SSL/TLS | Which sites present a valid cert. | Dedicated `/ssl?task=status`. /ssl?task=status same table. | **Partial** | No separate status API. |

### System Tools (2)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `jobs` → `/jobs` | System Tools | Inspect, retry, cancel background work. | Dedicated `/jobs`. Filters, inspect, retry; redacted payloads. | **Partial** | POST /jobs/{id}/cancel exists and is unused. No live poll. |
| `updates` → `/updates` | System Tools | Check/install signed Kelmor releases. | Dedicated `/updates`. Check/install signed feed + automatic flag. | **OK** | No release notes in API. |

### Themes (2)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `theme-manager` → `/section/system?tool=theme-manager` | Themes | Chrome density/favorites that render. | Catalog tool `/section/system?tool=theme-manager` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Density preference unused by CSS. | **Stub** |  |
| `customization` → `/section/system?tool=customization` | Themes | Logo/product/footer that render on chrome. | Catalog tool `/section/system?tool=customization` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Product name/footer unused by chrome. | **Stub** |  |

### Locales (1)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `locales` → `/section/system?tool=locales` | Locales | Display locale that renders. | Catalog tool `/section/system?tool=locales` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). en-US preference unused. | **Stub** |  |

### Development (2)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `api-tokens-whm` → `/accounts` | Development | Issue account-safe API tokens. | Dedicated `/accounts`. Deep-link intent is account services tokens tab. | **Partial** | Catalog path is /accounts not /accounts/:id/services?service=tokens. |
| `api-shell` → `/section/system?tool=api-shell` | Development | Documented control API (OpenAPI). | Catalog tool `/section/system?tool=api-shell` → `WhmToolBody` (`status`). Links to /openapi and /openapi.yaml. | **OK** | Not an interactive API shell. |

### Support (3)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `support-center` → `/section/system?tool=support-center` | Support | Collect logs/jobs/audit for a case. | Catalog tool `/section/system?tool=support-center` → `WhmToolBody` (`status`). How-to: hostname, Jobs, Audit, api.jsonl. | **Stub** | Honest. |
| `grant-support-access` → `/section/system?tool=grant-support-access` | Support | Issue a time-limited operator session. | Catalog tool `/section/system?tool=grant-support-access` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Ticket id + hours stored. No time-limited admin session issuer. | **Stub** |  |
| `diagnostics-log` → `/section/system?tool=diagnostics-log` | Support | Download a diagnostics bundle — or document collection. | Catalog tool `/section/system?tool=diagnostics-log` → `WhmToolBody` (`status`). Collection steps; no download bundle API. | **Stub** | Honest. |

### Multi Account Functions (2)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `multi-modify` → `/section/accounts?tool=multi-modify` | Multi Account Functions | Apply one package to many accounts via PATCH loop. | Catalog tool `/section/accounts?tool=multi-modify` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Usernames + package plan only. No bulk PATCH. | **Stub** |  |
| `multi-ip` → `/section/accounts?tool=multi-ip` | Multi Account Functions | Assign one IP to many accounts via PATCH loop (once IP apply works). | Catalog tool `/section/accounts?tool=multi-ip` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). Usernames + IP plan only. | **Stub** |  |

### cPanel (5)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `feature-showcase` → `/section/system?tool=feature-showcase` | cPanel | Release notes for new Director tools. | Catalog tool `/section/system?tool=feature-showcase` → `WhmToolBody` (`status`). Generic status description of “new tools”. | **Stub** | No release grouping. |
| `change-log` → `/updates` | cPanel | Operator notes for the installed channel. | Dedicated `/updates`. Opens /updates. | **OK** | Honest pointer. |
| `web-template-editor` → `/section/system?tool=web-template-editor` | cPanel | Default index/error pages copied into new docroots. | Catalog tool `/section/system?tool=web-template-editor` → `WhmToolBody` (`settings`). Save → `PATCH /api/v1/server/settings` (unconsumed JSON). index/error HTML unused. Provision uses /etc/skel. | **Stub** |  |
| `convert-addon` → `/section/system?tool=convert-addon` | cPanel | Promote an addon domain to a new POSIX account. | Catalog tool `/section/system?tool=convert-addon` → `WhmToolBody` (`wizard`). Wizard Save writes settings. Does not POST /accounts. | **Stub** |  |
| `reset-mailman` → `/email?tab=lists` | cPanel | Reset list admin password or edit membership. | Dedicated `/email?tab=lists`. /email?tab=lists. | **Partial** | Cannot rotate a “list password”; lists are aliases. |

### Plugins (1)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `plugins` → `/section/system?tool=plugins` | Plugins | Enable first-party host packages Kelmor already wires. | Catalog tool `/section/system?tool=plugins` → `WhmToolBody` (`status`). HostAppsPanel kind=plugin — review/enable wired host apps. | **Partial** | Not a third-party plugin loader. |

### Market (1)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `market` → `/section/system?tool=market` | Market | Install first-party apps (phpMyAdmin, Roundcube, WordPress). | Catalog tool `/section/system?tool=market` → `WhmToolBody` (`status`). HostAppsPanel kind=market: phpMyAdmin, Roundcube, WordPress enable. | **OK** | First-party apps only. |

### Extra dedicated / chrome routes (not catalog ids)

| Nav / URL | Category | Should do (WHM-class) | Current implementation | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `/accounts/:id` | Account Information | Account ops hub (not only the list picker). | `AccountSummaryPage`. Routed from list, Find, and `?task=` aliases. Not a separate catalog id (catalog uses `/accounts?task=summary`). | **Partial** | Same gaps as account-summary: PATCH/password job links; `?task=suspension` not auto-opened. |
| `/accounts/:id/services` | Account Information | Leftover account services that are not dedicated managers (SSH, tokens, backups, apps). | `AccountServicesPage`. Hub services redirect via `canonicalAccountToolPath`. Writes POST/DELETE leftover endpoints; “Track in Activity” without job id. | **Partial** | No catalog entry. `api-tokens-whm` still points at `/accounts`. |
| `/import` | Transfers | Same as Transfer or Restore. | Alias route → `TransfersPage`. | **OK** | Not in `whm-catalog.ts`; keep or redirect-document. |
| `/monitor` | Account Information | Same as View Bandwidth Usage. | Alias route → `UsagePage`. | **Partial** | Not in catalog. Usage page itself is Partial. |
| `/section/:hubId` | Kelmor Director | Category landing that does not dump tabs onto dedicated managers. | `HubPage`. Shows sibling catalog only here (`shouldShowHubTabs`). Dedicated default tools redirect to the manager. | **OK** | PR #38 / P0: dedicated routes stay free of hub tab dumps. |
| `/tools/:toolId` | Kelmor Director | Resolve a catalog id to the operator href. | `ToolRedirect` → `hrefForFeature` (section tool or dedicated path). Unknown id → Tool not found. | **OK** | Deep links and tests use this. |


---

## 7. Ranked remediation backlog (Director only)

Do **not** implement this list in the audit PR. Control app work is out of scope except keeping Login to Control HTTPS.

### P0 — Stop lying, finish job follow-through

| Rank | Item | Why | Suggested direction |
| ---: | --- | --- | --- |
| 1 | `change-site-ip`, `assign-ipv6`, `manage-shell` | Apply persists fields the worker ignores. | Either teach reconcile/Agent to apply IP + shell, or demote the tiles to honest status and remove Apply. |
| 2 | `unsuspend-bandwidth` | Unsuspends every suspended account. | Filter bandwidth holds only, or rename and confirm the blast radius. |
| 3 | `file-dir-restore` | Apply returns “Nothing to apply.” | Wire `POST /accounts/{id}/restores` with a path, or drop the button and link Transfers/services. |
| 4 | Job feedback on List suspend, Summary PATCH/password, Domains, Websites, Transfers, Account Services | APIs already return `operation_id`. | Reuse `QueuedOpNotice` / `jobsHref`. |
| 5 | Jobs cancel | `POST /jobs/{id}/cancel` is tested in Go and absent in the SPA. | Capability-gated cancel on queued/failed rows; keep retry. |

### P1 — Named tools operators will click next

| Rank | Item | Why | Suggested direction |
| ---: | --- | --- | --- |
| 6 | Quota / bandwidth tiles | After PR #38 they `PATCH package_id` (real). Still named like per-account overrides. | Keep the honesty banner or alias them to Change Package. Do not add byte overrides unless a new API exists. |
| 7 | Mailbox password/quota | Email manager is otherwise the real product. | New `PATCH` mailbox API, then UI. Until then keep the stub banner. |
| 8 | `multi-modify`, `multi-ip`, `change-ownership-bulk`, `convert-addon`, `ip-migration`, `synchronize-dns`, `dns-cleanup` | Confirm/wizard Saves are settings. | Loop real account/DNS APIs after review, or hide behind Preferences. |
| 9 | Feature Manager writes | Read-only `GET /feature-sets`. | Write API or keep **Stub** and stop implying assign-here. |
| 10 | Default sidebar density | 74 local-settings tiles still sit beside live tools. | Collapse “Preferences” by default; dedicated + API-backed stay first. |

### P2 — Real host settings worth building (only if product wants them)

| Rank | Item | Why | Suggested direction |
| ---: | --- | --- | --- |
| 11 | Custom PEM / service SSL | AutoSSL-only; `task=service` is copy. | Upload API or keep labeled Missing. |
| 12 | MultiPHP INI + nginx/tweak defaults | Settings blob vs `ApplyPhpPool` / vhost templates. | Agent fields or drop the forms. |
| 13 | Hostname, resolvers, host access, 2FA, cPHulk | Operators expect these to work. | Prefer extending `/security` + auth APIs over new settings keys. |
| 14 | Audit server-side filters | Page ignores API `q` / `since` / pagination. | Pass query params; raise the 200-event cap. |
| 15 | Cron edit/disable; file archive/extract | Dedicated pages are close. | Existing cron/files APIs + Agent pack ops if present. |
| 16 | SQL credentials engine + phpPgAdmin | Credentials GET is hardcoded `mariadb`. | Pass selected engine; label PG admin as unsupported. |

### P3 — Depth (do not block P0)

Reseller usage/IP delegation, usage graphs, process auto-refresh, security advisor scores, deliverability Fix, webmail per-mailbox SSO, release notes, chrome theme/locale, clusters/market extras, module installers.

**Out of product:** billing, copied WHM/cPanel UI, freeform root shell, Windows, Kubernetes, Control rewrite (except Director SSO already shipped).

---

## 8. File index

| Path | Role |
| --- | --- |
| `portals/server/src/whm-catalog.ts` | 191-feature catalog |
| `portals/server/src/nav-hubs.ts` | Hubs, dedicated vs `/section/?tool=` |
| `portals/server/src/operator-nav.ts` | Sidebar category order |
| `portals/server/src/app.tsx` | Dedicated routes |
| `portals/server/src/pages/whm-tool-page.tsx` | Generic apply |
| `portals/server/src/catalog-honesty.ts` | Local-settings + quota copy |
| `portals/server/src/control-url.ts` / `admin-tool-url.ts` | HTTPS Control / webmail / phpMyAdmin |
| `internal/httpserver/director_settings.go` | Unconsumed settings blob |
| `internal/httpserver/api.go` | Account PATCH, jobs cancel, impersonate |
| `internal/httpserver/portal_urls.go` | Public Control / admin-tool URLs |
| `docs/director-ui.md` | Operator chrome doc |
| `docs/director-whm-journey-checklist.md` | P0 checklist (does not inventory every tile) |

---

## 9. Deferred / omitted (cannot be a real host product yet)

These tiles stay honest **Settings (local)** / policy records. Do not pretend Agent apply.

| Audit id | Reason |
| --- | --- |
| `external-auth` | No OIDC/SAML login stack. |
| `two-factor` | No TOTP enroll/verify API. `require_2fa` is stored only. |
| `configuration-cluster`, `link-nodes`, `remote-access-key` | No Director cluster or peer-token product. |
| `module-installers`, `perl-modules`, `php-pear`, `php-pecl`, `ruby-gems` | No typed Agent op to apt-install arbitrary language modules. |
| `mysql-upgrade` | Major MariaDB upgrade is a planned record, not a live upgrade job. |
| `grant-support-access` | Ticket id note only; no time-boxed operator session issuer. |
| `server-profile` | Mail/DNS-only service hide is chrome policy, not a host role switch. |

Chrome tiles `theme-manager`, `locales`, `customization` apply to Director HTML (`data-density`, `lang`, product name) and do not queue `host.config.apply`.

---

## 10. What this document is not

- Not an implementation PR.
- Not a Control audit (see the companion portal-vs-panel map).
- Not a promise that every WHM name will become an Agent method.
- Statuses are from static review of `e72e6d8`. Re-dump `hrefForFeature` + `applyFeature` if the catalog moves.

Re-verify against `git log` if this file is more than a few weeks old.
