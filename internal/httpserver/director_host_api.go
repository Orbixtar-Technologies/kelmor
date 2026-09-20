package httpserver

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hosting-panel/panel/internal/dnsinventory"
	"github.com/hosting-panel/panel/internal/hostconfig"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/pkg/validate"
	"github.com/hosting-panel/panel/internal/rbac"
	"github.com/hosting-panel/panel/internal/store"
)

func (a *API) settingsFile() hostconfig.File {
	root := ""
	sockEmpty := true
	if a.Agent != nil {
		root = a.Agent.Root
		sockEmpty = a.Agent.Sock == ""
	}
	f, err := hostconfig.Load(hostconfig.StateDir(os.Getenv("PANEL_STATE_DIR"), root, sockEmpty))
	if err != nil {
		return hostconfig.File{Values: map[string]map[string]string{}}
	}
	return f
}

func (a *API) enqueueTypedJob(r *http.Request, job *store.Job, audit store.AuditEvent) (*store.Job, error) {
	if job.State == "" {
		job.State = "queued"
	}
	job.ActorID = actor(r).UserID
	job.RequestID = logging.RequestID(r.Context())
	if job.IdempotencyKey == "" {
		job.IdempotencyKey = r.Header.Get("Idempotency-Key")
	}
	return a.Store.EnqueueJobWithAudit(job, audit)
}

func (a *API) clearBandwidthHolds(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.AccountsSuspend) {
		return
	}
	var operations []string
	for _, acc := range a.Store.ListAccounts("", "") {
		if !actor(r).CanAccount(acc.ID) {
			continue
		}
		if !a.accountBandwidthHeld(&acc) {
			continue
		}
		usage := a.Store.GetUsage(acc.ID)
		if usage == nil {
			usage = &store.Usage{AccountID: acc.ID, CollectedAt: time.Now().UTC()}
		}
		usage.BandwidthBytes = 0
		usage.BandwidthHold = false
		usage.CollectedAt = time.Now().UTC()
		a.Store.PutUsage(usage)
		cp := acc
		job, err := a.enqueueBandwidthReset(&cp, r)
		if err != nil {
			continue
		}
		operations = append(operations, job.ID)
	}
	writeJSON(w, 202, map[string]any{"operations": operations, "cleared": len(operations)})
}

func (a *API) accountBandwidthHeld(acc *store.Account) bool {
	if acc == nil {
		return false
	}
	pkg := a.Store.GetPackage(acc.PackageID)
	u := a.Store.GetUsage(acc.ID)
	if pkg == nil || u == nil || pkg.BandwidthBytesMonthly <= 0 {
		return u != nil && u.BandwidthHold
	}
	return u.BandwidthHold || u.BandwidthBytes >= pkg.BandwidthBytesMonthly
}

func (a *API) bulkModifyAccounts(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.AccountsModify) {
		return
	}
	var in struct {
		Usernames  string `json:"usernames"`
		PackageID  string `json:"package_id"`
		ResellerID string `json:"reseller_id"`
		IPAddress  string `json:"ip_address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid bulk request", false)
		return
	}
	ids := a.accountIDsFromUsernames(in.Usernames)
	if len(ids) == 0 {
		a.fail(w, r, 400, "VALIDATION", "usernames is required", false)
		return
	}
	var operations []string
	for _, aid := range ids {
		acc := a.Store.GetAccount(aid)
		if acc == nil || !actor(r).CanAccount(aid) {
			continue
		}
		if in.PackageID != "" {
			acc.PackageID = in.PackageID
		}
		if in.ResellerID != "" {
			acc.ResellerID = in.ResellerID
		}
		if in.IPAddress != "" {
			if err := validateAccountIP(in.IPAddress, false); err != nil {
				a.fail(w, r, 400, "VALIDATION", err.Error(), false)
				return
			}
			acc.IPAddress = in.IPAddress
		}
		acc.DesiredRevision++
		job, err := a.Store.UpdateAccountWithJobAndAudit(acc, &store.Job{
			Type: "account.reconcile", ResourceType: "account", ResourceID: acc.ID,
			Payload: map[string]any{"account_id": acc.ID, "target_revision": acc.DesiredRevision},
			State:   "queued", ActorID: actor(r).UserID, RequestID: logging.RequestID(r.Context()),
		}, a.auditEvent(r, acc.ID, "account.bulk.modify", "account", acc.ID, nil, map[string]any{"package_id": acc.PackageID}))
		if err != nil {
			continue
		}
		operations = append(operations, job.ID)
	}
	writeJSON(w, 202, map[string]any{"operations": operations})
}

func (a *API) accountIDsFromUsernames(raw string) []string {
	want := map[string]bool{}
	for _, part := range strings.Fields(strings.ReplaceAll(raw, ",", " ")) {
		part = strings.ToLower(strings.TrimSpace(part))
		if part != "" {
			want[part] = true
		}
	}
	var ids []string
	for _, acc := range a.Store.ListAccounts("", "") {
		if want[strings.ToLower(acc.Username)] {
			ids = append(ids, acc.ID)
		}
	}
	return ids
}

func (a *API) migrateAccountIPs(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.AccountsModify) {
		return
	}
	var in struct {
		FromIP string `json:"from_ip"`
		ToIP   string `json:"to_ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid IP migration", false)
		return
	}
	if err := validateAccountIP(in.ToIP, false); err != nil {
		a.fail(w, r, 400, "VALIDATION", err.Error(), false)
		return
	}
	from := strings.TrimSpace(in.FromIP)
	var operations []string
	for _, acc := range a.Store.ListAccounts("", "") {
		if !actor(r).CanAccount(acc.ID) {
			continue
		}
		if strings.TrimSpace(acc.IPAddress) != from {
			continue
		}
		acc.IPAddress = strings.TrimSpace(in.ToIP)
		acc.DesiredRevision++
		job, err := a.Store.UpdateAccountWithJobAndAudit(&acc, &store.Job{
			Type: "account.reconcile", ResourceType: "account", ResourceID: acc.ID,
			Payload: map[string]any{"account_id": acc.ID, "target_revision": acc.DesiredRevision},
			State:   "queued", ActorID: actor(r).UserID, RequestID: logging.RequestID(r.Context()),
		}, a.auditEvent(r, acc.ID, "account.ip.migrate", "account", acc.ID, map[string]any{"from": from}, map[string]any{"to": acc.IPAddress}))
		if err != nil {
			continue
		}
		operations = append(operations, job.ID)
	}
	writeJSON(w, 202, map[string]any{"operations": operations})
}

func (a *API) listDNSSynchronize(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.DNSRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": dnsinventory.ManagedZones(a.Store)})
}

func (a *API) listDNSCleanup(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.DNSRead) {
		return
	}
	writeJSON(w, 200, map[string]any{"items": dnsinventory.LeftoverZones(a.Store)})
}

func (a *API) synchronizeDNS(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.DNSWrite) {
		return
	}
	payload := map[string]any{"scope": "all", "target": "all"}
	if ids := readZoneIDs(r); len(ids) > 0 {
		payload["zone_ids"] = ids
	}
	job, err := a.enqueueTypedJob(r, &store.Job{
		Type: "dns.synchronize", ResourceType: "dns",
		Payload: payload,
	}, a.auditEvent(r, "", "dns.synchronize", "dns", "", nil, map[string]any{"target": "all"}))
	if err != nil {
		a.fail(w, r, 500, "DNS_SYNC_ERROR", "Could not queue DNS synchronize", false)
		return
	}
	writeJSON(w, 202, map[string]any{"operation_id": job.ID})
}

func (a *API) cleanupDNS(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.DNSWrite) {
		return
	}
	payload := map[string]any{"scope": "terminated", "target": "orphans"}
	if ids := readZoneIDs(r); len(ids) > 0 {
		payload["zone_ids"] = ids
	}
	job, err := a.enqueueTypedJob(r, &store.Job{
		Type: "dns.cleanup", ResourceType: "dns",
		Payload: payload,
	}, a.auditEvent(r, "", "dns.cleanup", "dns", "", nil, map[string]any{"target": "orphans"}))
	if err != nil {
		a.fail(w, r, 500, "DNS_CLEANUP_ERROR", "Could not queue DNS cleanup", false)
		return
	}
	writeJSON(w, 202, map[string]any{"operation_id": job.ID})
}

func readZoneIDs(r *http.Request) []string {
	var in struct {
		ZoneIDs []string `json:"zone_ids"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	out := make([]string, 0, len(in.ZoneIDs))
	for _, id := range in.ZoneIDs {
		if strings.TrimSpace(id) != "" {
			out = append(out, strings.TrimSpace(id))
		}
	}
	return out
}

func (a *API) requireAny(w http.ResponseWriter, r *http.Request, caps ...string) bool {
	ac := actor(r)
	for _, cap := range caps {
		if ac.Has(cap) {
			return true
		}
	}
	a.fail(w, r, 403, "FORBIDDEN", "Missing capability", false)
	return false
}

func (a *API) notifyMail(w http.ResponseWriter, r *http.Request) {
	if !a.requireAny(w, r, rbac.ServerSettingsWrite, rbac.AccountsRead) {
		return
	}
	var in struct {
		From     string `json:"from"`
		Subject  string `json:"subject"`
		Body     string `json:"body"`
		Audience string `json:"audience"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid notify request", false)
		return
	}
	if strings.TrimSpace(in.Subject) == "" || strings.TrimSpace(in.Body) == "" || strings.TrimSpace(in.From) == "" {
		a.fail(w, r, 400, "VALIDATION", "from, subject, and body are required", false)
		return
	}
	if in.Audience == "" {
		in.Audience = "owners"
	}
	job, err := a.enqueueTypedJob(r, &store.Job{
		Type: "mail.notify", ResourceType: "mail",
		Payload: map[string]any{
			"from": in.From, "subject": in.Subject, "body": in.Body,
			"audience": in.Audience, "target": in.Audience,
		},
	}, a.auditEvent(r, "", "mail.notify", "mail", "", nil, map[string]any{"subject": in.Subject, "target": in.Audience}))
	if err != nil {
		a.fail(w, r, 500, "MAIL_NOTIFY_ERROR", "Could not queue notification mail", false)
		return
	}
	writeJSON(w, 202, map[string]any{"operation_id": job.ID})
}

func (a *API) convertAddon(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.AccountsCreate) {
		return
	}
	var in struct {
		AccountID     string `json:"account_id"`
		AddonDomain   string `json:"addon_domain"`
		Username      string `json:"username"`
		PackageID     string `json:"package_id"`
		OwnerPassword string `json:"owner_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid convert request", false)
		return
	}
	source := a.Store.GetAccount(in.AccountID)
	if source == nil || !actor(r).CanAccount(in.AccountID) {
		a.fail(w, r, 404, "NOT_FOUND", "Source account not found", false)
		return
	}
	ascii, err := validate.NormalizeDomain(in.AddonDomain)
	if err != nil {
		a.fail(w, r, 400, "VALIDATION", err.Error(), false)
		return
	}
	var sourceDomain *store.Domain
	for _, d := range a.Store.ListDomains(source.ID) {
		if d.ASCII == ascii && d.Type != "primary" {
			cp := d
			sourceDomain = &cp
			break
		}
	}
	if sourceDomain == nil {
		a.fail(w, r, 400, "VALIDATION", "Addon domain was not found on the source account", false)
		return
	}
	if msg := a.passwordPolicyError(in.OwnerPassword); msg != "" {
		a.fail(w, r, 400, "VALIDATION", msg, false)
		return
	}
	if err := validate.Username(in.Username); err != nil {
		a.fail(w, r, 400, "VALIDATION", err.Error(), false)
		return
	}
	if a.Store.AccountByUsername(in.Username) != nil || a.Store.UserByUsername(in.Username) != nil {
		a.fail(w, r, 400, "VALIDATION", "New username is missing or already taken", false)
		return
	}
	if a.Store.GetPackage(in.PackageID) == nil {
		a.fail(w, r, 400, "VALIDATION", "Unknown package", false)
		return
	}
	a.releaseConvertedAddon(source.ID, sourceDomain)
	created, operationID := a.createAccountFromConvert(w, r, in.Username, ascii, in.PackageID, source.ResellerID, in.OwnerPassword)
	if created == "" {
		return
	}
	out := map[string]any{"resource_id": created, "username": in.Username, "primary_domain": ascii}
	if operationID != "" {
		out["operation_id"] = operationID
	}
	writeJSON(w, 202, out)
}

func (a *API) releaseConvertedAddon(accountID string, sourceDomain *store.Domain) {
	for _, site := range a.Store.ListWebsites(accountID) {
		if site.DomainID == sourceDomain.ID {
			a.Store.DeleteWebsite(site.ID)
		}
	}
	a.Store.DeleteDomain(sourceDomain.ID)
}

func (a *API) createAccountFromConvert(w http.ResponseWriter, r *http.Request, username, domain, packageID, resellerID, ownerPassword string) (string, string) {
	body := map[string]any{
		"username": username, "primary_domain": domain, "package_id": packageID,
		"owner_email": "owner@" + domain, "owner_password": ownerPassword,
	}
	if resellerID != "" {
		body["reseller_id"] = resellerID
	}
	raw, _ := json.Marshal(body)
	r2 := r.Clone(r.Context())
	r2.Body = nopCloser{strings.NewReader(string(raw))}
	rec := &captureWriter{header: http.Header{}}
	a.createAccount(rec, r2)
	if rec.status != 0 && rec.status != http.StatusAccepted && rec.status != http.StatusOK && rec.status != http.StatusCreated {
		writeJSON(w, rec.status, rec.body)
		return "", ""
	}
	resourceID := ""
	operationID := ""
	if id, ok := rec.body["resource_id"].(string); ok {
		resourceID = id
	}
	if id, ok := rec.body["operation_id"].(string); ok {
		operationID = id
	}
	if resourceID == "" {
		if acc, ok := rec.body["account"].(map[string]any); ok {
			if id, ok := acc["id"].(string); ok {
				resourceID = id
			}
		}
	}
	return resourceID, operationID
}

type nopCloser struct{ *strings.Reader }

func (nopCloser) Close() error { return nil }

type captureWriter struct {
	status int
	header http.Header
	body   map[string]any
	raw    []byte
}

func (c *captureWriter) Header() http.Header { return c.header }

func (c *captureWriter) Write(p []byte) (int, error) {
	c.raw = append(c.raw, p...)
	_ = json.Unmarshal(c.raw, &c.body)
	return len(p), nil
}

func (c *captureWriter) WriteHeader(status int) { c.status = status }

func validateAccountIP(value string, allowEmpty bool) error {
	value = strings.TrimSpace(value)
	if value == "" {
		if allowEmpty {
			return nil
		}
		return errInvalidIP
	}
	if net.ParseIP(value) == nil {
		return errInvalidIP
	}
	return nil
}

var errInvalidIP = &simpleError{"ip_address must be a valid IPv4 or IPv6 address"}

type simpleError struct{ msg string }

func (e *simpleError) Error() string { return e.msg }

func (a *API) rejectDemoWrite(w http.ResponseWriter, r *http.Request, username string) bool {
	if hostconfig.IsDemoUsername(a.settingsFile(), username) && !actor(r).IsServerScope {
		a.fail(w, r, 403, "DEMO_ACCOUNT", "Demo accounts block destructive owner writes", false)
		return true
	}
	return false
}

func (a *API) passwordPolicyError(password string) string {
	return hostconfig.PasswordPolicyFrom(a.settingsFile()).Check(password)
}

func backupReady(b store.BackupRun) bool {
	switch b.State {
	case "succeeded", "complete", "ok":
		return true
	case "failed", "queued", "running":
		return false
	default:
		return b.Manifest != nil
	}
}

func latestAccountBackupID(st store.Store, accountID string) string {
	items := st.ListBackups(accountID)
	var best *store.BackupRun
	for i := range items {
		b := items[i]
		if !backupReady(b) {
			continue
		}
		if best == nil || b.CreatedAt.After(best.CreatedAt) {
			cp := b
			best = &cp
		}
	}
	if best == nil {
		return ""
	}
	return best.ID
}

func (a *API) enqueueHostConfigJob(r *http.Request, keys []string) (*store.Job, error) {
	// jobs.resource_id and audit_events.resource_id are UUIDs on Postgres.
	// The singleton host-config target is not a UUID, so leave ResourceID
	// empty (NULL) and keep the logical name in the payload.
	return a.enqueueTypedJob(r, &store.Job{
		Type: "host.config.apply", ResourceType: "server",
		Payload: map[string]any{"keys": keys, "target": "host-config"},
	}, a.auditEvent(r, "", "host.config.apply", "server", "", nil, map[string]any{"keys": keys}))
}

func sessionTTL(settings hostconfig.File) time.Duration {
	if minutes := hostconfig.IdleMinutes(settings); minutes > 0 {
		return time.Duration(minutes) * time.Minute
	}
	return 12 * time.Hour
}
