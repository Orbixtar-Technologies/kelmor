---
title: "feat: Build a WHM-style Kelmor Director"
date: 2026-09-09
artifact_contract: ce-unified-plan/v1
artifact_readiness: implementation-ready
execution: code
product_contract_source: ce-plan-bootstrap
---

# WHM-style Kelmor Director

## Goal Capsule

Transform Kelmor Director from a dark, sparse administration SPA into a
complete, light, polished server-operations panel whose information
architecture, feature breadth, and operator journeys are familiar to WHM
administrators while retaining Kelmor branding and Kelmor's capability-gated
API-to-agent privilege boundary.

## Problem Frame

The current Director places host metrics and destructive host actions at the
center of the experience. Its flat navigation and inline forms do not expose
the breadth already available in the Kelmor API. Operators cannot move through
account, package, reseller, DNS, service, backup, job, and audit work using the
grouped tools, dense lists, review steps, and contextual actions expected from
a mature server panel.

## Product Contract

### Actors

- **A1 — Server administrator:** manages the host, all accounts, packages,
  resellers, services, transfers, security, and audit data.
- **A2 — Server operator:** monitors services, accounts, jobs, usage, and audit
  data within granted capabilities.
- **A3 — Reseller:** manages only assigned accounts and packages using the same
  Director journeys, with all visibility and actions constrained by existing
  capabilities and account membership.

### Requirements

- **R1 — Kelmor chrome:** Use Kelmor Director branding only. Provide a light
  top bar, global Find, notifications, hostname, administrator menu,
  categorized collapsible navigation, category filtering, breadcrumbs, and
  responsive behavior.
- **R2 — WHM-style home:** Show favorites, host vitals, account/job statistics,
  service monitoring, recent operational activity, and grouped tools.
  Firewall and reboot remain available under security/server configuration,
  not as primary home calls to action.
- **R3 — Account information:** Provide searchable/filterable account,
  suspended-account, over-quota, and summary views with dense tables and
  contextual actions.
- **R4 — Account functions:** Provide a multi-step create-account wizard with
  review, modification, package change, suspend/unsuspend, termination
  confirmation, login/password controls, and package-limit visibility.
- **R5 — Packages and resellers:** Provide complete create, edit, and safe
  delete/package-assignment journeys plus reseller creation, editing, and
  privilege configuration.
- **R6 — Account-linked operations:** Surface DNS, SQL, email, SSL, websites,
  usage, files, backups, and related jobs through account-aware pages using
  existing Kelmor endpoints.
- **R7 — Host operations:** Provide service health, vitals, processes,
  firewall details/apply flow, and typed reboot confirmation as secondary
  administration tools.
- **R8 — Transfers and backups:** Provide native import/export, extracted
  archive migration, per-account backup creation, backup history, and restore
  review flows.
- **R9 — Jobs and audit:** Provide searchable/filterable/paginated dense
  histories, detail inspection, failed-job retry, and useful empty states that
  retain filters and explanatory context.
- **R10 — Capability integrity:** Hide unavailable tools and actions in the
  Director while preserving API enforcement for every read and mutation.
- **R11 — Quality evidence:** Add tests and a WHM-to-Kelmor checklist, and
  capture before/after screenshots for review.

### Key Flows

- **F1 — Find and act:** Press `/`, search a tool, username, or domain, select a
  result, and arrive at the tool or account summary.
- **F2 — Create account:** Open Account Functions, enter identity and ownership,
  choose package/reseller, inspect limits, review all values, submit, and follow
  the returned operation to Jobs.
- **F3 — Diagnose account:** Find an account, inspect summary/status/usage,
  navigate to linked DNS/email/SQL/SSL tools, and perform a capability-allowed
  action with feedback.
- **F4 — Change account state:** Select suspend, unsuspend, password rotation,
  package change, or termination from the account table/summary; require review
  or explicit destructive confirmation; then expose the queued operation.
- **F5 — Operate host:** Use Home status to enter Service Status or Security,
  inspect measured state, and run firewall/reboot actions only after review.
- **F6 — Investigate work:** Filter Jobs or Audit, open a detailed record, and
  retry failed work when the job is eligible.

### Acceptance Examples

- **AE1:** An administrator can navigate the primary operator workflows without
  encountering a dark theme, flat navigation, or an idle full-page stub.
- **AE2:** Global Find returns both tools and accounts and can be focused with
  `/` outside an editable field.
- **AE3:** Account creation cannot submit before the review step and sends the
  exact reviewed values to `POST /api/v1/accounts`.
- **AE4:** Destructive account and host actions require an explicit typed or
  checked confirmation before the API call.
- **AE5:** A reseller cannot discover or mutate a foreign account through the
  UI or API.
- **AE6:** Jobs and audit remain useful with no rows by showing filters,
  summaries, and guidance rather than replacing the whole page with an idle
  message.

## Scope Boundaries

### In scope

- `portals/server` information architecture, visual system, typed data models,
  reusable components, and all listed journeys.
- Focused Go API/store additions required for package lifecycle, reseller
  modification, account password rotation, and retrying failed jobs.
- Frontend logic tests, API integration tests, operator documentation, and
  visual evidence.

### Out of scope

- Copying cPanel or WHM trademarks, logos, proprietary assets, or source.
- Replacing Kelmor's authentication, RBAC, durable job model, or privileged
  agent architecture.
- Adding billing, arbitrary root-shell access, or unsupported service mutations.
- Redesigning Kelmor Control.

## Architecture and Data Flow

1. `portals/server/src/app.tsx` remains the authenticated route boundary and
   capability provider, but delegates chrome and pages to focused modules.
2. `portals/server/src/layout/` owns the top bar, categorized sidebar,
   breadcrumbs, command-style global Find, and route outlet.
3. `portals/server/src/components/` owns reusable page headers, status badges,
   tables, filters, dialogs, wizard steps, metrics, empty states, and feedback.
4. `portals/server/src/pages/` owns tool journeys. Account-scoped operation
   pages load the selected account and call existing account subresources.
5. `portals/server/src/types.ts` defines API entities. `client.ts` remains the
   authenticated fetch boundary and gains query/error helpers only as needed.
6. New backend mutations are registered in
   `internal/httpserver/api.go`, validated there, capability-checked with
   existing RBAC constants, persisted through `internal/store/store.go`, and
   covered by both memory/PostgreSQL implementations.
7. Host-impacting password rotation and retry work is represented as durable
   jobs. No portal code writes system state directly.

## Key Technical Decisions

- **KTD1 — No new UI framework:** Build the polished interface with React,
  React Router, CSS, and lightweight inline SVG icons. This avoids introducing
  a design-system dependency into a small Vite portal while still creating a
  reusable component layer.
- **KTD2 — Route-backed tools:** Every sidebar/global-find tool targets a real
  route. Related account resources share an account operations hub rather than
  creating decorative placeholder pages.
- **KTD3 — Client pagination for bounded APIs:** Jobs and audit APIs currently
  return bounded lists. Filtering and pagination are performed client-side for
  this slice; API state filtering remains in use for Jobs. This provides the
  complete operator flow without changing store query contracts unnecessarily.
- **KTD4 — Safe lifecycle endpoints:** Package deletion is rejected while
  assigned to accounts. Failed-job retry clones the operation into a fresh
  queued job rather than mutating immutable history. Account password rotation
  updates the owner login hash and queues account reconciliation with the Linux
  password in the existing secret-stripping job flow.
- **KTD5 — Capability-derived discovery:** Navigation, global Find results, and
  action controls share capability metadata so an unavailable operation is not
  discoverable in chrome even though the API remains authoritative.

## Implementation Units

### U1 — Foundation, chrome, and discovery

Files:
- `portals/server/src/app.tsx`
- `portals/server/src/types.ts`
- `portals/server/src/tool-catalog.ts`
- `portals/server/src/layout/director-shell.tsx`
- `portals/server/src/layout/global-find.tsx`
- `portals/server/src/layout/sidebar.tsx`
- `portals/server/src/components/*`
- `portals/server/src/styles.css`

Work:
- Introduce typed API entities and a capability-aware tool catalog.
- Implement light Kelmor chrome, categorized/collapsible navigation, category
  filter, breadcrumbs, notifications, admin menu, and keyboard search.
- Add reusable dense table, status, page, dialog, pagination, and form
  components.

Tests:
- `portals/server/src/tool-catalog.test.ts`
- `portals/server/src/layout/global-find.test.tsx`

Scenarios:
1. Tool discovery excludes entries whose capability is unavailable.
2. Find matches tool labels, account usernames, and domains.
3. `/` focuses Find unless focus is already in an editable control.
4. Sidebar category filtering and expand/collapse preserve reachable tools.

### U2 — Home and host operations

Files:
- `portals/server/src/pages/home-page.tsx`
- `portals/server/src/pages/service-status-page.tsx`
- `portals/server/src/pages/security-page.tsx`

Work:
- Build favorites, vitals, stats, service monitoring, activity, and grouped
  tool sections.
- Add service/process tables and secondary firewall/reboot review flows.

Tests:
- Browser smoke scenarios in `scripts/ui-mvp.sh` or the repository's browser
  harness.

Scenarios:
1. Measured values and services render with loading/error states.
2. Reboot requires the `REBOOT` confirmation value.
3. Firewall apply is hidden without capability and reports API feedback.

### U3 — Account information and functions

Files:
- `portals/server/src/pages/accounts-page.tsx`
- `portals/server/src/pages/create-account-page.tsx`
- `portals/server/src/pages/account-summary-page.tsx`
- `portals/server/src/pages/account-operations-page.tsx`
- `portals/server/src/account-wizard.ts`

Work:
- Add list/suspended/over-quota views, filters, sorting, selection, and actions.
- Build the multi-step account wizard and review submission.
- Add summary, package change, owner/login controls, explicit terminate
  confirmation, and linked DNS/email/SQL/SSL/usage/backup sections.

Tests:
- `portals/server/src/account-wizard.test.ts`
- `internal/httpserver/api_test.go`

Scenarios:
1. Wizard validation blocks invalid usernames/domains and missing package.
2. Review data exactly matches submitted API data.
3. Filters distinguish suspended and over-quota accounts.
4. Password rotation validates strength, marks forced rotation when selected,
   updates owner credentials, and queues Linux reconciliation.
5. Termination does not call the API until the username confirmation matches.

### U4 — Packages and resellers

Files:
- `portals/server/src/pages/packages-page.tsx`
- `portals/server/src/pages/resellers-page.tsx`
- `internal/httpserver/api.go`
- `internal/store/store.go`
- `internal/store/memory.go`
- `internal/store/postgres.go`
- `internal/httpserver/api_test.go`

Work:
- Add complete package limit forms, edit, safe delete, and assignment visibility.
- Add reseller wizard/editing for identity, status, nameservers, and privileges.
- Add capability-gated lifecycle routes and store delete methods.

Tests:
- Package update persists all supported limit fields.
- Assigned packages cannot be deleted.
- Unassigned package delete succeeds and is audited.
- Reseller modification enforces `resellers.modify` and persists privileges.

### U5 — Transfers, jobs, audit, and account-linked tools

Files:
- `portals/server/src/pages/transfers-page.tsx`
- `portals/server/src/pages/jobs-page.tsx`
- `portals/server/src/pages/audit-page.tsx`
- `portals/server/src/pages/dns-page.tsx`
- `portals/server/src/pages/account-services-page.tsx`
- `internal/httpserver/api.go`
- `internal/httpserver/api_test.go`

Work:
- Present import/migration/export and backup/restore as reviewable journeys.
- Add full Jobs and Audit filters, pagination, details, and useful empty states.
- Add retry endpoint that creates a fresh queued job from an eligible failed
  operation while retaining original history and audit provenance.
- Group existing DNS, database, mail, certificate, website, file, cron, SSH,
  FTP, token, and usage APIs into account-linked operational tabs.

Tests:
- Failed job retry creates a new queued job with safe payload and provenance.
- Non-failed jobs and inaccessible jobs cannot be retried.
- Filter/pagination helpers return stable result sets.

### U6 — Documentation and visual verification

Files:
- `docs/director-whm-journey-checklist.md`
- `README.md`

Work:
- Map each requested WHM pattern to its Kelmor route and API status.
- Document local Director verification and distinguish real endpoints from thin
  queued operations.
- Capture the rejected screenshot as before evidence and browser-verified light
  Director pages as after evidence for the pull request.

## Error Handling and Security

- Show loading, empty, and error states within page structure so navigation and
  filters remain usable.
- Treat API messages as plain text; never render server-provided HTML.
- Validate account, domain, password, numeric limit, and confirmation input on
  both client and server where mutations are added.
- Keep capability checks at route, navigation, control, and API layers.
- Keep job payload secrets out of responses and remove Linux passwords after
  agent dispatch using the existing worker behavior.
- Reject package deletion when any account references the package.
- Clone only retry-safe stored payloads and preserve actor/request provenance
  in the new job and audit event.

## Verification Strategy

1. Install only test dependencies required by the portal using the latest
   compatible versions.
2. Run portal unit tests and `npm run build --prefix portals/server`.
3. Run focused `go test ./internal/httpserver ./internal/store`.
4. Run `make lint`, `make test`, and the existing security suite if setup
   permits.
5. Start the local API and Director through the documented development path.
6. Browser-test login, global Find, home, list accounts, create-account review,
   account summary, packages, resellers, jobs, audit, and destructive
   confirmations at desktop and narrow viewport widths.
7. Save before/after screenshots and reference them in the PR description.

## Risks and Mitigations

- **Large frontend rewrite:** Keep modules focused, use shared primitives, and
  verify routes incrementally with build and browser checks.
- **Existing API shape variability:** Normalize nullable list responses through
  `asList` and use typed optional fields for measured host data.
- **Store interface expansion:** Implement memory and PostgreSQL methods in the
  same unit and cover behavior at the HTTP boundary.
- **Visual similarity vs. trademark copying:** Reproduce operator patterns and
  information architecture, not logos, proprietary icons, names, or assets.
- **External WHM research unavailable:** The required `parallel-cli` was not
  installed. The implementation is therefore grounded in the explicit user
  acceptance bar and repository API evidence, with no unsupported external
  claims.

## Requirements Traceability

- R1, F1, AE2 → U1
- R2, R7, F5 → U2
- R3, R4, F2–F4, AE3–AE5 → U3
- R5 → U4
- R6, R8, R9, F6, AE6 → U5
- R10 → U1–U5
- R11, AE1 → U6

Product Contract unchanged after technical enrichment.
