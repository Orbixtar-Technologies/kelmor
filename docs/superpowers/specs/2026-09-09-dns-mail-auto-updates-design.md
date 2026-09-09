# DNS, Mail, and Automatic Updates Design

## Goal

Restore reliable DNS synchronization and public mail-host resolution, then add
secure unattended Kelmor updates from a configurable signed HTTPS feed.

## Current failures

The worker sends DNS mutations to PowerDNS at `127.0.0.1:8081`. On an existing
Ubuntu installation, `applyDNS` preserves the packaged `pdns.conf` and only
reconciles bind-backend settings. It does not add the API listener settings, so
the worker receives connection-refused errors.

The `orbixtar.dpdns.org` zone and its mail A/MX records resolve through
PowerDNS locally. Public recursive resolvers return `SERVFAIL` because the
delegated authorities are unreachable. The intended authorities,
`ns1.kelmor.host` and `ns2.kelmor.host`, currently return `NXDOMAIN` from the
parent-managed `kelmor.host` zone.

The existing updater verifies Ed25519 signatures and artifact hashes and keeps
a binary rollback snapshot. It does not fetch releases, compare versions,
update portals, schedule checks, restart services, validate health, expose
status through the API, or roll back failed service restarts.

## DNS and mail repair

The installer will reconcile all Kelmor-owned PowerDNS settings in an existing
configuration:

- `bind-config=/etc/powerdns/named.conf`
- `bind-dnssec-db=/var/lib/panel/dns/bind-dnssec.sqlite3`
- `local-address=<loopback and detected host addresses>`
- `local-port=53`
- `webserver=yes`
- `webserver-address=127.0.0.1`
- `webserver-port=8081`
- `api=yes`
- `api-key=panel-loopback`

The packaged parent `launch=` line remains untouched because Ubuntu activates
the bind backend through `pdns.d/bind.conf` with `launch+=bind`. Reconciliation
must be idempotent and replace active or commented conflicting settings
without duplicating directives.

DNS phase verification will require the expected configuration. Live host
verification will additionally require a successful authenticated request to
the loopback PowerDNS API. This prevents a completed install phase from hiding
the same failure.

Mail-domain provisioning will retain the existing local records and verify
that the zone includes:

- an A record for `mail.<domain>` using the account or public server address;
- an MX record targeting that mail host;
- SPF, DKIM, and DMARC TXT records;
- a deterministic, syntactically valid SOA and authoritative NS records.

The Director DNS page will show separate local-authority and public-delegation
states. A local zone can therefore be healthy while public delegation is
broken. Diagnostics will name unreachable or missing authorities rather than
reporting a generic synchronization failure.

Parent-zone delegation is operational work outside this repository. The
`kelmor.host` DNS provider must publish `ns1.kelmor.host` and
`ns2.kelmor.host` A/glue records for `150.239.113.59`, and delegated domains
must use reachable authorities. Kelmor will report this requirement but will
not claim to mutate a registrar without configured provider credentials.

## Release feed

Each channel has a base HTTPS URL configured in `/etc/panel/update.env`. The
stable feed serves:

```text
<base-url>/stable/manifest.json
<base-url>/stable/<release>/<artifact-path>
```

The manifest includes release version, channel, minimum compatible release,
artifact paths, sizes, SHA-256 hashes, and one Ed25519 signature over canonical
manifest content. The trusted public key is provisioned separately at
`/etc/panel/update.pub`; a manifest-provided key is never trusted.

The checker rejects:

- non-HTTPS production feeds;
- invalid signatures or hashes;
- channel mismatches;
- malformed or non-increasing versions;
- incompatible release jumps;
- absolute paths, traversal, symlinks, duplicate destinations, or unexpected
  artifact classes;
- files or total downloads above configured safety limits;
- redirects to a different scheme or unapproved host.

## Update lifecycle

A root-owned `panel-update.service` oneshot and
`panel-update.timer` perform daily checks with randomized delay. The service
uses a process lock so scheduled and manual checks cannot overlap.

The checker performs this sequence:

1. Load immutable local policy and the pinned public key.
2. Fetch and verify the channel manifest.
3. Compare it with `/usr/local/panel/current-release`.
4. Download artifacts into a private staging directory.
5. Verify every size and hash before changing the installation.
6. Snapshot current managed binaries, portals, units, and release metadata.
7. Install staged files using same-filesystem temporary files and atomic
   renames.
8. Reload systemd and restart affected Kelmor services.
9. Check API health, worker and agent activity, portal responses, and the
   PowerDNS API.
10. Mark the release active only after all checks pass.

Any installation or health-check failure restores the snapshot, reloads and
restarts the previous release, records the failure, and leaves the downloaded
release inactive. A failed rollback is recorded as a critical state and does
not trigger another automatic attempt until an administrator acknowledges it.

Automatic installation is enabled for the stable channel by default on new
installs. Existing installs receive the timer and stable policy when the
installer is rerun. Operators can disable automatic installation while
retaining periodic update checks.

## Privilege and API boundaries

Only the root-owned updater downloads and installs release artifacts. The
unprivileged API never accepts arbitrary feed URLs, public keys, filesystem
paths, or executable content.

The API exposes:

- `GET /api/v1/server/updates` for installed/latest version, channel,
  automatic-update policy, last check, last result, and rollback state;
- `POST /api/v1/server/updates/check` to trigger the configured checker;
- `POST /api/v1/server/updates/install` to install the already verified
  available release;
- `PATCH /api/v1/server/updates/settings` to enable or disable automatic
  installation and select an administrator-approved configured channel.

Read access requires `server.read`. Mutations require
`server.settings.write`, reject account-scoped tokens, and create audit events.
The API sends a typed `ManagePanelUpdate` operation to the root-owned agent.
That operation accepts only `check`, `install`, or the bounded automatic-update
setting, then starts the fixed systemd service or rewrites the fixed policy
file. It never executes a caller-supplied command.

Updater state is written atomically to `/var/lib/panel/update-status.json`.
It contains no feed credentials, signing secrets, or private operational data.

## Director experience

“Software Updates” appears under System Tools for users with `server.read`.
The page displays the installed and latest releases, channel, last successful
check, automatic-install state, current operation, and any verification or
rollback error.

Users with `server.settings.write` can check now, install a verified release,
and toggle automatic installation. Installing requires a confirmation that
states the target version. Controls are disabled while an operation is active,
and status messages are announced to assistive technology.

## Tests

Implementation follows test-first development:

1. Installer tests begin with Ubuntu’s packaged PowerDNS configuration and
   prove all API directives are reconciled once without changing `launch=`.
2. DNS tests cover expected mail records and distinguish local health from
   missing public delegation.
3. Updater unit tests cover canonical signatures, version ordering, URL and
   path validation, size limits, staging, atomic installation, health failure,
   rollback, locking, and status persistence.
4. API tests cover capabilities, account-scoped token rejection, audit events,
   fixed service requests, and invalid settings.
5. React tests cover status rendering, permissions, confirmation, loading,
   success, and accessible errors.
6. A live deployment check verifies PowerDNS API access, retries failed DNS
   jobs, queries the zone locally and publicly, checks SMTP/IMAP listeners and
   TLS names, and exercises a signed no-op update feed before enabling the
   production feed.

## Deployment and operations

The implementation will be packaged, installed on the existing Ubuntu host,
and verified without erasing accounts or zones. Existing failed DNS jobs will
be retried after PowerDNS API health is established.

Public DNS and ACME verification remains blocked until the parent DNS records
for the authoritative nameservers are published. The panel will expose that
blocker explicitly while local DNS and mail services remain operational.
