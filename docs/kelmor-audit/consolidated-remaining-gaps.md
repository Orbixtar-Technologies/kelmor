# Kelmor Director — consolidated remaining gaps

Follow-up live review on 10 September 2026. This document adds the gaps found after the earlier database, account, jobs, search, mobile, credential, and backup reviews.

## New high-priority gaps

1. **Database credentials are visible by default.** The dedicated Database Manager exposes a database password in the connection-details panel. Mask it by default; require an explicit reveal or copy action and preserve access controls. The value is intentionally omitted from this report and screenshots.

2. **Destructive host controls lack visible guardrails.** Security shows `Request reboot` beside a warning that active sites and sessions will disconnect, but no maintenance window, typed confirmation, impact summary, or scheduled-reboot option is visible. Firewall has `Review and apply`, but the page does not expose a before/after diff, validation result, rollback path, or change scope before that action.

3. **Account scope is unclear on shared tools.** DNS, SSL, Files, Email, Webmail, and Database Manager use account selectors, but their breadcrumbs and headings often show only the tool name. Keep the selected account visible in the page title or a persistent context chip, with a clear account-change action and return-to-account link.

4. **DNS editing is too destructive and under-explained.** Records expose Delete without visible edit, duplicate, or rollback actions. The zone badge says `INACTIVE` beside `Enable DNSSEC`, but does not say whether the zone or DNSSEC is inactive. Add explicit state labels, record editing, validation by record type, change preview, and a safe undo/history path.

5. **Audit data is dense and difficult to interpret.** The trail lists internal action names, UUID-like actor/resource identifiers, source IP, and separate intent/install events. Group related events into one human-readable operation, expose outcome reasons, and make the default table readable before requiring Inspect. Verify whether source IP should be shown to every operator role.

## New medium-priority gaps

6. **File Manager exposes sensitive and operational directories without a safety model.** The home listing shows `.ssh`, backups, logs, mail, and panel metadata alongside web content. Add a clear protected-path warning, permission/read-only indicators, search and sorting, and a safer distinction between account content and system-managed files. Rename/Delete actions should be visually separated from Edit and require contextual confirmation.

7. **File Manager lacks list controls for growth.** There is no visible sort, filter, pagination, selected-item bulk action, or file-size summary. The table will become difficult to use as an account grows. Add these before relying on the current flat directory listing.

8. **Email and Webmail have inconsistent management depth.** Email Management shows mailbox quota and status but only Delete; Webmail adds Open webmail and Manage. Consolidate mailbox details, password reset, quota editing, and access state into one predictable resource workflow.

9. **Webmail connection guidance is partly placeholder content.** The URL is shown as `https://webmail.<domain>/` and the copy says it opens when configured. Display the resolved URL when available, explain configuration state, and provide copy controls for IMAP/SMTP settings. Avoid making an operator infer whether `mail.hosting` is a placeholder or live host.

10. **Certificate management hides lifecycle and failure detail.** Certificates show ACTIVE and an absolute expiration date, but no days remaining, renewal state, covered names, issuer, or last renewal result. Add expiry urgency, SAN coverage, renewal history, and a clear action for a failed request.

11. **Transfers and backups need stronger review boundaries.** Native import, extracted archive import, account copy, and export are presented together, but the page does not show file size, source trust, previewed object counts, collision impact, or rollback/cleanup behavior. Put each journey behind a review summary with explicit scope and expected queued jobs.

12. **Software Updates lacks release notes and scheduling context.** The page shows installed and available versions, channel, automatic install state, and last check, but no release notes, maintenance window, rollback plan, signature verification result, or next automatic-install time. The disabled install action should explain why it is unavailable when installed and available versions match.

13. **Server Status presents equally strong controls for unequal risk.** reload, restart, start, and stop are all small inline links with equal visual weight. Make the safe read-only state primary, distinguish disruptive actions, show confirmation impact and resulting job status, and provide last action/time per service.

14. **Loading and refresh behavior needs explicit state communication.** Several routes briefly show `Loading data…` and then replace it without a visible last-updated time or retry affordance. Add stable skeletons, a timeout/error state, last refresh time, and a retry action; preserve the selected account/service while refreshing.

## Existing gaps confirmed again

The original findings remain unresolved in this review: multiple sidebar items look active, service tabs overflow, mobile job tables hide actions behind horizontal scrolling, account-linked job counts and visible rows disagree in scope, raw JSON is used for errors, resource rows prioritize Delete over management, narrow account cards wrap identifiers, and the summary duplicates host metrics.

## Evidence captured

The new screenshots are in [the consolidated screenshot folder](C:/Users/usman/Documents/Codex/2026-09-10/ana/outputs/kelmor-audit/consolidated). The earlier evidence remains in [the original audit](C:/Users/usman/Documents/Codex/2026-09-10/ana/outputs/kelmor-audit/kelmor-design-audit.md) and [the previous remaining-gaps report](C:/Users/usman/Documents/Codex/2026-09-10/ana/outputs/kelmor-audit/remaining/remaining-gaps.md).

Inspected in this follow-up: DNS, Email Management, Webmail, SSL/TLS, File Manager, Server & Service Status, Security & Host Configuration, Audit Trail, Transfers & Backups, and Software Updates. No create, delete, restart, reboot, firewall apply, import/export, backup, update, credential reveal, or other state-changing action was performed. Full role-based access, keyboard traversal, contrast measurement, screen-reader semantics, confirmation dialogs, and every responsive breakpoint remain untested.
