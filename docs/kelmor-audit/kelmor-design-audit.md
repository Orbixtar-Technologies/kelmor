# Kelmor Director — design, UI/UX and layout review

Reviewed live on 10 September 2026 at https://lab.kelmor.host:8443.

## Overall assessment

A coherent, conventional hosting administration interface with a useful operational foundation. Its strongest qualities are consistent cards, restrained colors, explicit account identity, and recognizable status indicators. Its main weaknesses are navigation ambiguity, too many competing navigation layers, weak handling of narrower widths, and interfaces organized around forms and internal operations rather than the operator’s next task. Targeted refinement is more appropriate than a wholesale visual replacement.

## Scope and evidence

Read-only review of the open database screen, account summary, recent account jobs, website services, home dashboard, and the shared navigation/resource shell. Captures include the naturally available narrow (~683 px) and wide (~1523 px) browser layouts; this was not a controlled breakpoint sweep. No data, service configuration, credentials, or account state was changed. The original database URL was restored. All seven screenshots below were saved and inspected; transient loading captures were replaced with loaded states.

## Layout structure

The wide view has a dark left navigation rail of approximately 250 px, a top utility bar of approximately 58 px, a breadcrumb band, a central account workspace, and a right server-resource rail of approximately 275 px. The central workspace contains the page heading and actions, account section links, service tabs, and a bordered service card holding an inline form above a table. Account summary uses a two-column card grid. Home uses metric cards, operational shortcuts, and a grid of administration tools.

At the narrow width, the global navigation collapses into a hamburger and the resource rail is absent from the visible layout. However, the account summary still uses two columns, and the service navigation becomes a horizontally scrolling strip. Global chrome adapts more successfully than the content within the page.

## Visual design

The navy sidebar, light gray workspace, white panels, blue primary buttons, and green/red status badges form a consistent administrative palette. Borders and modest corner rounding make grouping clear. Large page headings establish the first level of hierarchy.

Secondary labels, table content, breadcrumbs, and helper text are noticeably small and faint. There is considerable empty panel area alongside compact data rows, giving an uneven density: spacious containers with compressed content. Service tab counts have visibly stretched backgrounds in the narrow capture. Home shortcut icons vary in color and visual treatment; a more consistent icon family and sizing system would improve polish. Keep the core palette and focus on type, spacing, state clarity, and content priority.

## Prioritized improvements

| Priority | Finding | Evidence | Recommended change |
|---|---|---|---|
| High | Multiple sidebar destinations appear active | Step 7 | Highlight only the current destination; distinguish parent groups visually. Make account-level and server-level scope explicit. |
| High | Fourteen peer service tabs are hard to scan and overflow | Steps 1, 5, 7 | Group services into Web, Email, Data, Access, and Automation; use a labeled selector at narrow widths. Keep counts compact. |
| High | Existing resources have no visible management entry beyond Delete | Steps 1, 5, 7 | Make resource names open details; prioritize routine supported actions. Put creation behind one clear primary action. |
| High | Narrow summary layout breaks domains and addresses awkwardly | Step 2 | Stack cards sooner; let identifiers use available width and provide a copy affordance where useful. |
| High | Failure feedback exposes implementation details | Step 3 | Present the failed task, affected resource, readable reason, and details/recovery entry. Keep raw diagnostics expandable. |
| Medium | Summary prioritizes editable configuration over account health | Steps 3, 4 | Lead with status, domain, plan, quota usage, and attention items. Move configuration changes into an explicit edit flow. |
| Medium | Host metrics are duplicated and momentarily disagree | Step 6 | Consolidate metrics, distinguish host/account scope, and show update freshness. Make the resource rail collapsible where it crowds the task. |
| Medium | Copy and action labels are overly technical or vague | Steps 3–6 | Replace capability-enforced API language with what happens and how users track progress. Clarify Apply website and Reconciliation. |
| Medium | Density and active-state styling lack balance | Steps 1–7 | Reduce redundant chrome, strengthen secondary text, use consistent form gaps, and clarify the active account section. |

## Proposed information hierarchy

1. Global shell: server/context indicator, search, notifications, profile; one clear active sidebar destination.
2. Account header: account name, primary domain, status, and concise account actions.
3. Account sections: Overview, Services, DNS, Activity, and Settings, subject to actual product capabilities.
4. Service selection: grouped destinations on wide layouts, a labeled selector on narrower layouts.
5. Service workspace: service name and count, one primary create action, and an existing-resource list with a details entry. Add search/filtering as list size warrants it.
6. Secondary information: contextual health and recent operations; host resources should not displace essential account controls.

## Accessibility observations and limits

Visible labels and accessible names exist for the observed inputs and top-bar controls. Service tabs expose selection semantics, and status badges contain words rather than relying only on color. However, the observed table header cells are exposed as generic cells, so verify proper header associations. The breadcrumb exposes Home as a link while intermediate account context is plain text; make useful ancestors navigable. Multiple active-looking navigation entries also create orientation problems beyond color alone. Small secondary text, compact deletion controls, and narrow-width wrapping warrant contrast, target-size, zoom, and reflow checks.

No numerical contrast measurement, keyboard traversal, focus-order audit, screen-reader test, 200% zoom test, controlled mobile test, or submit/delete confirmation test was performed. Search, notifications, empty states, and complete create/update workflows were not exercised. No accessibility compliance or full-site functional claim is made. Suggested actions require validation against supported product behavior.

## Captured review steps

### 1. Databases, narrow view

Health: **Needs improvement**. The selected service is identifiable, and inputs have visible labels. Fourteen service destinations overflow horizontally; count badges stretch into tall pills. Page chrome occupies roughly the first 390 pixels before the service card begins. Replace the long strip at narrow widths with a labeled service selector and compact counts.

![Step 1: Databases, narrow view](C:/Users/usman/Documents/Codex/2026-09-10/ana/outputs/kelmor-audit/01-databases.png)

### 2. Account summary, narrow view

Health: **Needs improvement**. The account identity and primary action are clear. The two-column summary persists at about 680 pixels, forcing the domain and IP address to break awkwardly. Stack cards earlier and use more balanced label/value widths.

![Step 2: Account summary, narrow view](C:/Users/usman/Documents/Codex/2026-09-10/ana/outputs/kelmor-audit/02-account-summary.png)

### 3. Account usage, security, and recent jobs

Health: **Needs improvement**. Textual status badges communicate success and failure independently of color. The failed job exposes raw JSON and an internal operation name with no visible per-row recovery link. Use a plain-language error, affected resource, time, and a details action. Separate permanent termination from routine credential maintenance. Confirmation behavior was not tested.

![Step 3: Account usage, security, and recent jobs](C:/Users/usman/Documents/Codex/2026-09-10/ana/outputs/kelmor-audit/03-account-jobs.png)

### 4. Account summary, wide view

Health: **Usable with hierarchy issues**. Cards align consistently, and Manage services is easy to identify. Package editing occupies a full top-level card while usage appears below the initial viewport. Make the summary status-first, show usage and failures early, and place less frequent editing behind an Edit action. Reconciliation 2 / 2 needs an explanation.

![Step 4: Account summary, wide view](C:/Users/usman/Documents/Codex/2026-09-10/ana/outputs/kelmor-audit/04-summary-wide.png)

### 5. Websites, wide view

Health: **Needs improvement**. The service panel uses the same form and table structure as databases. Apply website is ambiguous about whether it creates or updates a website; the existing row exposes only Delete. Explain the action and provide a website details/edit entry point. Manage DNS is duplicated by the adjacent DNS section and is not specific to the current service.

![Step 5: Websites, wide view](C:/Users/usman/Documents/Codex/2026-09-10/ana/outputs/kelmor-audit/05-websites-wide.png)

### 6. Home dashboard

Health: **Usable with redundant content**. The dashboard offers recognizable administration shortcuts and visible health statistics. Load, memory, disk, and account counts are repeated in the resource rail. At capture, load reads 0.00 in the main cards and 0.03 in the rail; this may reflect different refresh times, not a backend error. Use one authoritative presentation or show freshness explicitly.

![Step 6: Home dashboard](C:/Users/usman/Documents/Codex/2026-09-10/ana/outputs/kelmor-audit/06-home.png)

### 7. Databases, wide view and return to original route

Health: **Needs improvement**. The wide layout makes the three-column shell clear. Many sidebar items simultaneously have the same blue background and bold emphasis, including List Accounts, Suspended Accounts, Modify an Account, and Terminate an Account. Users cannot reliably infer current location. Use one current destination and a distinct parent-group style. The database row offers Delete but no visible detail or management action.

![Step 7: Databases, wide view and return to original route](C:/Users/usman/Documents/Codex/2026-09-10/ana/outputs/kelmor-audit/07-databases-wide.png)

