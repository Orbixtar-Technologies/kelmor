# DNS, Mail, and Automatic Updates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Repair PowerDNS synchronization and deploy a signed, health-checked
automatic update path with Director visibility.

**Architecture:** The installer converges PowerDNS and update-system
configuration on both fresh and existing Ubuntu hosts. A root-owned updater
fetches a pinned-key HTTPS feed, stages and verifies releases, applies them
atomically, and rolls back failed health checks. The API reaches it only
through a typed privileged-agent operation and exposes read-only status to the
Director.

**Tech Stack:** Go 1.23, systemd, PowerDNS Authoritative API, React 19,
TypeScript 5.9, Vitest, Ed25519, SHA-256.

## Global Constraints

- Production releases require HTTPS and a separately pinned Ed25519 public key.
- Automatic installation uses the `stable` channel.
- Update callers cannot supply commands, URLs, keys, or filesystem paths.
- Existing accounts, zones, mailboxes, and installation state must survive.
- Public DNS remains explicitly degraded until parent-zone delegation exists.
- Preserve the user's unrelated lockfile and generated-file changes.

---

### Task 1: Converge PowerDNS on upgrades

**Files:**
- Modify: `installer/phases/stack.go`
- Modify: `installer/phases/stack_test.go`

**Interfaces:**
- Consumes: existing `replaceConfigLine(path, prefix, replacement)` helper.
- Produces: `reconcilePowerDNSConfig(path, listen string) error`.

- [ ] **Step 1: Write the failing upgrade regression test**

Create a packaged Ubuntu-style `pdns.conf` with commented API defaults and
assert that two calls to `applyDNS` produce exactly one active line for every
Kelmor-owned API directive while preserving the parent `launch=` line.

```go
func TestApplyDNSReconcilesPowerDNSAPIOnUpgrade(t *testing.T) {
	// Arrange the packaged configuration under the dev install root.
	// Act twice to prove convergence.
	// Assert api=yes, webserver=yes, loopback:8081, panel-loopback key,
	// bind paths, and one copy of each directive.
}
```

- [ ] **Step 2: Run the focused test and observe the expected failure**

Run:

```bash
go test ./installer/phases -run TestApplyDNSReconcilesPowerDNSAPIOnUpgrade -v
```

Expected: failure because the existing configuration still has `# api=no` and
no active API listener.

- [ ] **Step 3: Implement deterministic reconciliation**

Add a helper that replaces active or commented forms of owned directives and
appends a directive only when neither form exists:

```go
func reconcilePowerDNSConfig(path, listen string) error {
	settings := map[string]string{
		"bind-config":        "/etc/powerdns/named.conf",
		"bind-dnssec-db":     "/var/lib/panel/dns/bind-dnssec.sqlite3",
		"local-address":      listen,
		"local-port":         "53",
		"webserver":          "yes",
		"webserver-address":  "127.0.0.1",
		"webserver-port":     "8081",
		"api":                "yes",
		"api-key":            "panel-loopback",
	}
	return replacePowerDNSSettings(path, settings)
}
```

Call it from `applyDNS`; extend `verifyDNS` to require the active settings.

- [ ] **Step 4: Run installer tests**

```bash
go test ./installer/phases
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add installer/phases/stack.go installer/phases/stack_test.go
git commit -m "fix: reconcile PowerDNS API configuration"
```

### Task 2: Build the remote update client and transactional installer

**Files:**
- Create: `internal/update/client.go`
- Create: `internal/update/client_test.go`
- Create: `internal/update/status.go`
- Create: `internal/update/status_test.go`
- Modify: `internal/update/verify.go`
- Modify: `internal/update/verify_test.go`
- Modify: `internal/update/apply.go`
- Modify: `internal/update/apply_test.go`
- Modify: `cmd/panel-updater/main.go`

**Interfaces:**
- Produces: `Config`, `Status`, `Artifact`, expanded `Manifest`.
- Produces: `Check(ctx, config) (*Status, error)`.
- Produces: `Install(ctx, config, runner) (*Status, error)`.
- Produces: `WriteStatus(path string, status Status) error`.

- [ ] **Step 1: Write failing manifest and client validation tests**

Tests cover valid stable manifests, pinned-key verification, semantic version
ordering, HTTPS enforcement, same-host redirects, size bounds, and path
rejection.

```go
func TestCheckAcceptsNewerSignedStableRelease(t *testing.T) {}
func TestCheckRejectsHTTPFeed(t *testing.T) {}
func TestCheckRejectsTraversalAndOversizedArtifacts(t *testing.T) {}
func TestCheckRejectsNonIncreasingRelease(t *testing.T) {}
```

- [ ] **Step 2: Run update tests and observe missing interfaces**

```bash
go test ./internal/update -run 'TestCheck|TestManifest' -v
```

Expected: compile failure for the new `Config`, `Artifact`, and `Check`
interfaces.

- [ ] **Step 3: Implement feed and status models**

Use explicit artifact destinations rather than flattening every file into
`bin`:

```go
type Artifact struct {
	Path   string `json:"path"`
	Target string `json:"target"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode"`
}

type Config struct {
	FeedURL          string
	Channel          string
	InstalledRelease string
	PublicKey        ed25519.PublicKey
	InstallRoot      string
	StatusPath       string
	Automatic        bool
}

type Status struct {
	State            string `json:"state"`
	InstalledRelease string `json:"installed_release"`
	AvailableRelease string `json:"available_release,omitempty"`
	LastCheckedAt    string `json:"last_checked_at,omitempty"`
	Error            string `json:"error,omitempty"`
	Automatic        bool   `json:"automatic"`
	Channel          string `json:"channel"`
}
```

Canonical signatures include all compatibility and artifact fields. Resolve
artifact URLs under the configured channel/release path, enforce limits while
streaming, and persist status by temporary-file rename.

- [ ] **Step 4: Write failing transactional install tests**

```go
func TestInstallUpdatesBinariesAndPortals(t *testing.T) {}
func TestInstallRollsBackWhenHealthCheckFails(t *testing.T) {}
func TestInstallLockRejectsConcurrentOperation(t *testing.T) {}
```

- [ ] **Step 5: Run tests and confirm transactional behavior is absent**

```bash
go test ./internal/update -run 'TestInstall' -v
```

Expected: failures because portals, health checks, and operation locking are
not implemented.

- [ ] **Step 6: Implement staging, snapshots, health checks, and rollback**

`Install` downloads into a private temporary directory, verifies the complete
release, snapshots each allowed target, atomically renames staged files,
invokes a fixed runner for systemd reload/restart and health checks, and
restores the snapshot on any failure.

- [ ] **Step 7: Extend the updater CLI**

Support fixed commands and config:

```text
panel-updater check --config /etc/panel/update.env
panel-updater install --config /etc/panel/update.env
panel-updater run --config /etc/panel/update.env
panel-updater status
```

`run` checks and installs only when `PANEL_UPDATE_AUTOMATIC=true`.

- [ ] **Step 8: Run all updater tests**

```bash
go test ./internal/update ./cmd/panel-updater
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/update cmd/panel-updater
git commit -m "feat: add signed transactional update client"
```

### Task 3: Add the privileged update boundary and API

**Files:**
- Create: `agent/operations/update.go`
- Create: `agent/operations/update_test.go`
- Modify: `agent/operations/ops.go`
- Create: `internal/httpserver/updates.go`
- Create: `internal/httpserver/updates_test.go`
- Modify: `internal/httpserver/api.go`
- Modify: `api/openapi.yaml`

**Interfaces:**
- Produces typed agent method `ManagePanelUpdate`.
- Produces update status and mutation API routes.

- [ ] **Step 1: Write failing agent boundary tests**

```go
func TestManagePanelUpdateAcceptsFixedActions(t *testing.T) {}
func TestManagePanelUpdateRejectsCommandsURLsAndPaths(t *testing.T) {}
```

The accepted request shape is:

```go
struct {
	Action    string `json:"action"` // check, install, settings
	Automatic *bool  `json:"automatic,omitempty"`
}
```

- [ ] **Step 2: Run the agent tests and observe the unknown operation**

```bash
go test ./agent/operations -run TestManagePanelUpdate -v
```

Expected: failure because `ManagePanelUpdate` is not dispatched.

- [ ] **Step 3: Implement the fixed privileged operation**

For `check` and `install`, start the fixed `panel-update.service` with a
validated mode. For `settings`, atomically update only
`PANEL_UPDATE_AUTOMATIC=true|false` in `/etc/panel/update.env`. No shell is
used.

- [ ] **Step 4: Write failing API authorization and audit tests**

```go
func TestUpdateStatusRequiresServerRead(t *testing.T) {}
func TestUpdateMutationsRequireServerSettingsWrite(t *testing.T) {}
func TestUpdateMutationsRejectAccountScopedToken(t *testing.T) {}
func TestUpdateMutationIsAudited(t *testing.T) {}
```

- [ ] **Step 5: Run focused API tests and observe missing routes**

```bash
go test ./internal/httpserver -run TestUpdate -v
```

Expected: 404 responses.

- [ ] **Step 6: Implement status and mutation handlers**

Add the four routes from the design. Read status from the fixed state path,
validate settings JSON strictly, call `ManagePanelUpdate`, and audit successful
mutations without recording secrets.

- [ ] **Step 7: Run agent and API tests**

```bash
go test ./agent/operations ./internal/httpserver
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add agent/operations internal/httpserver api/openapi.yaml
git commit -m "feat: expose capability-protected server updates"
```

### Task 4: Install the updater and add Director controls

**Files:**
- Create: `installer/phases/units/panel-update.service`
- Create: `installer/phases/units/panel-update.timer`
- Modify: `installer/phases/stack.go`
- Modify: `installer/phases/stack_test.go`
- Modify: `packaging/debian/build.sh`
- Create: `portals/server/src/pages/updates-page.tsx`
- Create: `portals/server/src/pages/updates-page.test.tsx`
- Modify: `portals/server/src/app.tsx`
- Modify: `portals/server/src/tool-catalog.ts`

**Interfaces:**
- Installs and enables daily update checks.
- Adds `/updates` Director route and “Software Updates” tool.

- [ ] **Step 1: Write failing installer assertions**

Assert that installation creates a root-only update configuration, pinned
public-key path, oneshot unit, daily timer with randomized delay, and enables
the timer.

- [ ] **Step 2: Run installer tests and observe missing files**

```bash
go test ./installer/phases -run TestInstall -v
```

Expected: failure for absent update configuration and units.

- [ ] **Step 3: Implement update installation**

Install:

```ini
[Service]
Type=oneshot
ExecStart=/usr/local/panel/bin/panel-updater run --config /etc/panel/update.env
```

and:

```ini
[Timer]
OnBootSec=15min
OnUnitActiveSec=1d
RandomizedDelaySec=1h
Persistent=true
```

Write `PANEL_UPDATE_CHANNEL=stable`,
`PANEL_UPDATE_AUTOMATIC=true`, fixed status/lock/install paths, and the
configured feed URL. Never place the signing private key on the target.

- [ ] **Step 4: Write failing Director tests**

Use React Testing Library to verify status content, read-only permissions,
confirmation before installation, disabled controls while active, and
announced request errors.

- [ ] **Step 5: Run Director tests and observe the missing page**

```bash
npm test --prefix portals/server -- updates-page.test.tsx
```

Expected: module/route failure.

- [ ] **Step 6: Implement the updates page**

Add the named `UpdatesPage` component, `/updates` route guarded by
`server.read`, tool-catalog entry, capability-aware mutation controls, loading
state, and an alert/status region.

- [ ] **Step 7: Run installer and portal checks**

```bash
go test ./installer/phases
npm test --prefix portals/server
npm run build --prefix portals/server
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add installer packaging portals/server/src
git commit -m "feat: schedule updates and add Director controls"
```

### Task 5: Package, deploy, and prove recovery

**Files:**
- Modify only if verification exposes a tested defect.

**Interfaces:**
- Produces a bootstrapped VM able to consume later signed releases.

- [ ] **Step 1: Run full local verification**

```bash
go test ./...
go vet ./...
npm test --prefix portals/server
npm run build --prefix portals/server
npm run build --prefix portals/account
make package
```

Expected: all commands exit zero.

- [ ] **Step 2: Commit and push the verified implementation**

Stage only files owned by this plan, commit any final focused correction, and
push `cursor/dns-mail-auto-updates-1c4f`.

- [ ] **Step 3: Bootstrap release trust**

Generate an Ed25519 publisher key outside the VM, store only its public half at
`/etc/panel/update.pub`, and preserve the private half in the local gitignored
validation directory for transfer to the user’s secret store. Build and sign
the bootstrap feed, then serve it from the configured HTTPS update origin.

- [ ] **Step 4: Deploy the package**

Copy the Debian package and public update key to the VM, install it, rerun
`panel-install`, and restart the installed units. Do not copy the signing
private key.

- [ ] **Step 5: Verify PowerDNS and retry DNS jobs**

```bash
curl -H 'X-API-Key: panel-loopback' \
  http://127.0.0.1:8081/api/v1/servers/localhost
dig @127.0.0.1 mail.orbixtar.dpdns.org A
dig @127.0.0.1 orbixtar.dpdns.org MX
```

Retry failed `dns.sync` jobs through the authenticated API, then confirm they
succeed.

- [ ] **Step 6: Verify services and update automation**

```bash
systemctl is-active panel-agent panel-api panel-worker pdns postfix dovecot
systemctl is-enabled panel-update.timer
systemctl list-timers panel-update.timer
panel-updater check --config /etc/panel/update.env
curl -fsS http://127.0.0.1:18080/healthz
curl -kfsS https://127.0.0.1:8443/
curl -kfsS https://127.0.0.1:8444/
```

Expected: services active, timer enabled, signed feed accepted, API healthy,
and both portals available.

- [ ] **Step 7: Verify and report external delegation separately**

Query `1.1.1.1` for `ns1.kelmor.host`, `ns2.kelmor.host`, and
`mail.orbixtar.dpdns.org`. Report public success only if those independent
queries return authoritative answers; otherwise report the exact parent-zone
blocker.
