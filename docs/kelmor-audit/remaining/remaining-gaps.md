# Kelmor Director — remaining UI/UX gaps

Second live inspection, 10 September 2026. This supplements the first audit rather than repeating its navigation/highlight and visual-density findings.

## Most urgent additional finding: visible database credential

The dedicated Database Manager displayed a database password in plain text by default, alongside connection details. Mask it by default and use an explicit reveal/copy interaction with appropriate access controls. This is an observed display-exposure issue, not evidence of unauthorized access or a server compromise. The credential was not reproduced in this document, and that screen was deliberately excluded from saved screenshots.

The dedicated manager also offers connection information absent from the account-level database tab. Share a coherent resource-details experience across both routes, with clear scope and a consistent way to reach connection settings.

## Priority order

1. Mask credentials by default.
2. Fix account scope visibility and inconsistent Jobs totals.
3. Make mobile job identity and actions reachable without horizontal scrolling.
4. Fix error focus, requirement guidance, and account-creation autofill behavior.
5. Explain backup destination readiness, scope, retention, and recovery entry points.
6. Clarify search scope and unify Escape behavior.
7. Improve job-detail recovery guidance and reduce nested scrolling.
8. Refine account row actions, quota readability, and empty-state recovery.

## Verification boundaries

Inspected tool search and an exact resource-name query, notifications, the account table and suspended filter, first-step account validation, the dedicated database manager, backup empty state, account-linked jobs, job details, and mobile Jobs/navigation at a requested 390 × 844 viewport. Read-only navigation and local UI validation only: no creation, deletion, suspension, retry, backup, configuration save, or credential change was performed.

Not tested: full wizard completion, database/backup submission, destructive confirmations, unsaved-change protection with real edits, DNS/mail/file-manager/SSL workflows, permission variations, numerical contrast, full screen-reader behavior, 200% zoom, or every responsive breakpoint. Therefore this is a bounded additional audit, not a claim that every product gap has been found.

## Captured steps

### 1. Global tool search

Health: **Usable**. A database query returns Database Manager and Escape dismisses results. The input has a visible focus outline.

![Global tool search](C:/Users/usman/Documents/Codex/2026-09-10/ana/outputs/kelmor-audit/remaining/01-search.png)

### 2. Search scope

Health: **Gap**. Searching the existing database by its exact name returns no matching tools or accounts. Rename the field Search tools and accounts, or support resource search and explain scope.

![Search scope](C:/Users/usman/Documents/Codex/2026-09-10/ana/outputs/kelmor-audit/remaining/02-search-scope.png)

### 3. Notifications

Health: **Gap**. Escape did not dismiss this panel, whereas search, the job dialog, and mobile navigation did dismiss. Implement consistent dismissal and return focus to the opener. No unread alerts is not proof that there are no historical failed jobs.

![Notifications](C:/Users/usman/Documents/Codex/2026-09-10/ana/outputs/kelmor-audit/remaining/03-notifications.png)

### 4. Account list

Health: **Usable with gaps**. Manage and Suspend have very similar link styling despite different consequences. Disk quota shows a rounded 0% and used bytes without the quota limit. Display used/limit and group disruptive actions distinctly. Select-page remains exposed when the table is empty.

![Account list](C:/Users/usman/Documents/Codex/2026-09-10/ana/outputs/kelmor-audit/remaining/04-accounts.png)

### 5. Suspended-account empty state

Health: **Mostly usable**. The screen explains that no suspended accounts match and retains filters. Offer a direct Show all accounts action and distinguish no suspended accounts from no search matches.

![Suspended-account empty state](C:/Users/usman/Documents/Codex/2026-09-10/ana/outputs/kelmor-audit/remaining/05-empty-accounts.png)

### 6. Account creation validation

Health: **Gap**. Clicking Continue without completing identity fields shows associated inline errors, but focus remains on Continue; no visible error summary appears. Show required/optional status and username/password guidance before validation, and focus the first invalid input or a linked error summary. The browser appeared to autofill a login name into Owner email and a masked value into Initial owner password. Verify autocomplete=email and new-password behavior; HTML attributes were not inspected. No credentials were entered, revealed, or submitted by the auditor.

![Account creation validation](C:/Users/usman/Documents/Codex/2026-09-10/ana/outputs/kelmor-audit/remaining/06-form-validation.png)

### 7. Backup empty state

Health: **Gap**. Only a destination type and Queue encrypted backup are visible. No description of included data, retention, encryption-key handling, destination readiness, or route to restore/import appears. Explain these before queueing. This does not establish whether configuration exists elsewhere or whether restore controls appear once a backup exists.

![Backup empty state](C:/Users/usman/Documents/Codex/2026-09-10/ana/outputs/kelmor-audit/remaining/07-backups.png)

### 8. Account-linked Jobs

Health: **High-priority gap**. Related jobs retains the account query parameter but shows only Jobs and Home / Jobs in the page. There is no visible account chip, clear-account-filter control, or return-to-account link. The success metric reads 26 while the account-filtered table has five total results, four successful and one failed. Align card and list scope or label the wider totals explicitly; a backend counting defect is not established.

![Account-linked Jobs](C:/Users/usman/Documents/Codex/2026-09-10/ana/outputs/kelmor-audit/remaining/08-related-jobs.png)

### 9. Failed job details

Health: **Gap with working keyboard behavior**. A Retry failed job control exists. It was not invoked. The dialog emphasizes IDs, JSON, and repeated log text with nested scroll areas; its action footer is below the initial visible area. Summarize the cause and prerequisites for recovery, retain technical detail in expandable areas, and keep actions visible. Logs contain earlier DNS failures and a later missing-resource error, but no attempt timeline explains their relationship. Escape closes the dialog and restores focus to Details.

![Failed job details](C:/Users/usman/Documents/Codex/2026-09-10/ana/outputs/kelmor-audit/remaining/09-job-details.png)

### 10. Jobs at phone width

Health: **High-priority layout gap**. At a requested 390 × 844 viewport, four stacked metric cards consume much of the page before filters and results. The global search placeholder is clipped. Use compact metric chips or a two-column summary and prioritize the job list.

![Jobs at phone width](C:/Users/usman/Documents/Codex/2026-09-10/ana/outputs/kelmor-audit/remaining/10-jobs-mobile.png)

### 11. Phone-width job table

Health: **High-priority interaction gap**. Details, errors, attempts, and progress are beyond the initial horizontal viewport; the scrollbar is at the bottom of the table. Long resource IDs wrap into several lines. Use a compact job card or a pinned identity/action column so primary actions remain accessible without horizontal hunting.

![Phone-width job table](C:/Users/usman/Documents/Codex/2026-09-10/ana/outputs/kelmor-audit/remaining/11-jobs-mobile-table.png)

### 12. Mobile navigation

Health: **Mostly usable**. The drawer focuses Filter features on open. Escape closes it and returns focus to the navigation button. It retains its previous scroll position, so early destinations may be out of view; consider scrolling the current item into view. Full focus trapping and screen-reader modal semantics were not tested.

![Mobile navigation](C:/Users/usman/Documents/Codex/2026-09-10/ana/outputs/kelmor-audit/remaining/12-mobile-navigation.png)

### 13. Dedicated Database Manager

Health: **High-priority credential-display gap**. Screen inspected through the live accessible page content; screenshot intentionally omitted to avoid persisting the visible password. See the lead finding and cross-route consistency recommendation above.
