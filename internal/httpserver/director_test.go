package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/auth"
	"github.com/hosting-panel/panel/internal/id"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/rbac"
	"github.com/hosting-panel/panel/internal/store"
)

// directorFixture boots an API with the dev seed and returns an admin token.
// The Director operator journeys are all admin-scoped, so every test here
// starts from the same place an operator does after signing in.
func directorFixture(t *testing.T) (*httptest.Server, store.Store, string) {
	t.Helper()
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	t.Cleanup(srv.Close)
	login := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})
	return srv, st, login["token"].(string)
}

func seedAccount(t *testing.T, srv *httptest.Server, token, username, domain, packageID string) string {
	t.Helper()
	created := post(t, srv.URL+"/api/v1/accounts", token, map[string]string{
		"username": username, "primary_domain": domain, "package_id": packageID,
		"owner_email": "owner@" + domain, "owner_password": "TenantPass!2026",
	})
	return created["resource_id"].(string)
}

func firstPackageID(t *testing.T, srv *httptest.Server, token string) string {
	t.Helper()
	items := get(t, srv.URL+"/api/v1/packages", token)["items"].([]any)
	return items[0].(map[string]any)["id"].(string)
}

func TestFeatureSetsIndex(t *testing.T) {
	srv, st, token := directorFixture(t)
	items := get(t, srv.URL+"/api/v1/feature-sets", token)["items"].([]any)
	if len(items) != len(st.ListFeatureSets()) || len(items) == 0 {
		t.Fatalf("feature sets: %v", items)
	}
	if items[0].(map[string]any)["features"] == nil {
		t.Fatalf("feature set omitted feature flags: %v", items[0])
	}
}

func TestPackageLifecycleAndInUseGuard(t *testing.T) {
	srv, _, token := directorFixture(t)

	created := post(t, srv.URL+"/api/v1/packages", token, map[string]any{
		"name": "Business", "disk_bytes": 1 << 30, "bandwidth_bytes_monthly": 10 << 30, "cpu_percent": 100,
		"domains": 25,
	})
	pid := created["id"].(string)

	fetched := get(t, srv.URL+"/api/v1/packages/"+pid, token)
	if fetched["name"] != "Business" {
		t.Fatalf("get package: %v", fetched)
	}

	// The full record must round-trip, not only the handful of columns the
	// original upsert touched.
	updated := doJSON(t, http.MethodPut, srv.URL+"/api/v1/packages/"+pid, token, map[string]any{
		"name": "Business Plus", "disk_bytes": 4 << 30, "bandwidth_bytes_monthly": 40 << 30,
		"mailboxes": 250, "process_limit": 512, "email_daily_limit": 2000,
	})
	if updated["name"] != "Business Plus" {
		t.Fatalf("update name: %v", updated)
	}
	if updated["mailboxes"].(float64) != 250 || updated["process_limit"].(float64) != 512 {
		t.Fatalf("update did not persist every limit: %v", updated)
	}
	if updated["domains"].(float64) != 0 {
		t.Fatalf("PUT did not fully replace an omitted limit: %v", updated)
	}
	patched := doJSON(t, http.MethodPatch, srv.URL+"/api/v1/packages/"+pid, token, map[string]any{
		"name": "Business Plus Patched",
	})
	if patched["mailboxes"].(float64) != 250 || patched["process_limit"].(float64) != 512 {
		t.Fatalf("PATCH did not retain omitted limits: %v", patched)
	}

	seedAccount(t, srv, token, "inuse01", "inuse.test", pid)
	status, body := doStatus(t, http.MethodDelete, srv.URL+"/api/v1/packages/"+pid, token, nil)
	if status != http.StatusConflict {
		t.Fatalf("delete of in-use package: %d %v", status, body)
	}

	spare := post(t, srv.URL+"/api/v1/packages", token, map[string]any{"name": "Spare"})
	if status, body := doStatus(t, http.MethodDelete, srv.URL+"/api/v1/packages/"+spare["id"].(string), token, nil); status != http.StatusOK {
		t.Fatalf("delete unused package: %d %v", status, body)
	}
	if status, _ := doStatus(t, http.MethodGet, srv.URL+"/api/v1/packages/"+spare["id"].(string), token, nil); status != http.StatusNotFound {
		t.Fatalf("deleted package still readable: %d", status)
	}
}

func TestPackageDetailEnforcesResellerVisibility(t *testing.T) {
	srv, _, admin := directorFixture(t)
	first := post(t, srv.URL+"/api/v1/resellers", admin, map[string]any{
		"name": "First Reseller", "username": "first-reseller", "password": "ResellerPass!2026",
	})
	second := post(t, srv.URL+"/api/v1/resellers", admin, map[string]any{
		"name": "Second Reseller", "username": "second-reseller", "password": "ResellerPass!2026",
	})
	own := post(t, srv.URL+"/api/v1/packages", admin, map[string]any{
		"name": "First Plan", "reseller_id": first["id"],
	})
	foreign := post(t, srv.URL+"/api/v1/packages", admin, map[string]any{
		"name": "Second Plan", "reseller_id": second["id"],
	})
	resellerToken := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "first-reseller", "password": "ResellerPass!2026",
	})["token"].(string)
	if status, _ := doStatus(t, http.MethodGet, srv.URL+"/api/v1/packages/"+own["id"].(string), resellerToken, nil); status != http.StatusOK {
		t.Fatalf("own package detail status: %d", status)
	}
	if status, _ := doStatus(t, http.MethodGet, srv.URL+"/api/v1/packages/"+foreign["id"].(string), resellerToken, nil); status != http.StatusNotFound {
		t.Fatalf("foreign package detail should be hidden: %d", status)
	}
}

func TestResellerDetailAndPrivilegeMask(t *testing.T) {
	srv, _, token := directorFixture(t)

	created := post(t, srv.URL+"/api/v1/resellers", token, map[string]any{
		"name": "Northwind", "username": "northwind", "password": "ResellerPass!2026", "email": "ops@northwind.test",
	})
	rid := created["id"].(string)

	pid := firstPackageID(t, srv, token)
	aid := seedAccount(t, srv, token, "nwcust", "nwcust.test", pid)
	doJSON(t, http.MethodPatch, srv.URL+"/api/v1/accounts/"+aid, token, map[string]any{"reseller_id": rid})
	ownedPackage := post(t, srv.URL+"/api/v1/packages", token, map[string]any{
		"name": "Northwind Plan", "reseller_id": rid,
	})

	detail := get(t, srv.URL+"/api/v1/resellers/"+rid, token)
	if detail["reseller"].(map[string]any)["name"] != "Northwind" {
		t.Fatalf("detail: %v", detail)
	}
	owned := detail["accounts"].([]any)
	if len(owned) != 1 || owned[0].(map[string]any)["username"] != "nwcust" {
		t.Fatalf("owned accounts: %v", owned)
	}
	packages := detail["packages"].([]any)
	if len(packages) != 1 || packages[0].(map[string]any)["id"] != ownedPackage["id"] {
		t.Fatalf("owned packages: %v", packages)
	}
	owner := detail["owner"].(map[string]any)
	if owner["username"] != "northwind" {
		t.Fatalf("safe owner identity: %v", owner)
	}
	encoded, _ := json.Marshal(detail)
	if bytes.Contains(encoded, []byte("password_hash")) || bytes.Contains(encoded, []byte("$2")) {
		t.Fatalf("reseller detail leaked owner password hash: %s", encoded)
	}

	updated := doJSON(t, http.MethodPatch, srv.URL+"/api/v1/resellers/"+rid, token, map[string]any{
		"name": "Northwind Hosting", "status": "suspended", "privilege_mask": []string{"accounts.create", "backups.restore"},
	})
	if updated["name"] != "Northwind Hosting" || updated["status"] != "suspended" {
		t.Fatalf("update: %v", updated)
	}
	if len(updated["privilege_mask"].([]any)) != 2 {
		t.Fatalf("privilege mask: %v", updated)
	}

	// A mask may only narrow the reseller role, never widen it into server
	// administration.
	status, body := doStatus(t, http.MethodPatch, srv.URL+"/api/v1/resellers/"+rid, token, map[string]any{
		"privilege_mask": []string{"server.firewall.write"},
	})
	if status != http.StatusBadRequest {
		t.Fatalf("expected privilege escalation to be refused: %d %v", status, body)
	}
}

func TestJobRetryAndCancel(t *testing.T) {
	srv, st, token := directorFixture(t)
	pid := firstPackageID(t, srv, token)
	accountID := seedAccount(t, srv, token, "jobowner", "jobowner.test", pid)

	failed, _ := st.EnqueueJob(&store.Job{
		Type: "account.reconcile", ResourceType: "account", ResourceID: accountID,
		Payload: map[string]any{"account_id": accountID, "status": "active"},
		State:   "failed", Attempts: 3, MaxAttempts: 3, LastError: "agent unreachable",
	})
	before, _ := json.Marshal(st.GetJob(failed.ID))

	retried := post(t, srv.URL+"/api/v1/jobs/"+failed.ID+"/retry", token, nil)
	retryID, _ := retried["operation_id"].(string)
	retry := st.GetJob(retryID)
	if retry == nil || retry.ID == failed.ID || retry.State != "queued" ||
		retry.ResourceID != accountID || retry.Payload["retry_of"] != failed.ID {
		t.Fatalf("retry clone: response=%v job=%+v", retried, retry)
	}
	after, _ := json.Marshal(st.GetJob(failed.ID))
	if !bytes.Equal(before, after) {
		t.Fatalf("retry mutated original\nbefore: %s\nafter: %s", before, after)
	}

	cancelTarget, _ := st.EnqueueJob(&store.Job{
		Type: "account.reconcile", ResourceType: "account", ResourceID: accountID,
		Payload: map[string]any{"account_id": accountID, "linux_password": "DoNotExpose!2026"},
		State:   "failed", Attempts: 2, LastError: "kept for history", LockedBy: "stale-worker",
	})
	cancelled := post(t, srv.URL+"/api/v1/jobs/"+cancelTarget.ID+"/cancel", token, nil)
	if cancelled["state"] != "cancelled" {
		t.Fatalf("cancel state: %v", cancelled)
	}
	cancelledJSON, _ := json.Marshal(cancelled)
	if bytes.Contains(cancelledJSON, []byte("DoNotExpose")) ||
		bytes.Contains(cancelledJSON, []byte("linux_password")) {
		t.Fatalf("cancel response leaked sensitive payload: %s", cancelledJSON)
	}
	stored := st.GetJob(cancelTarget.ID)
	if stored.State != "cancelled" || stored.FinishedAt == nil || stored.LockedBy != "" ||
		stored.Attempts != 2 || stored.LastError != "kept for history" {
		t.Fatalf("cancel did not atomically preserve history and clear lock: %+v", stored)
	}
	if status, _ := doStatus(t, http.MethodPost, srv.URL+"/api/v1/jobs/"+cancelTarget.ID+"/cancel", token, nil); status != http.StatusConflict {
		t.Fatalf("cancelling an already cancelled job must conflict: %d", status)
	}

	running, _ := st.EnqueueJob(&store.Job{
		Type: "backup.create", ResourceType: "account", ResourceID: accountID,
		Payload: map[string]any{"account_id": accountID}, State: "queued",
	})
	running.State = "running"
	st.UpdateJob(running)
	if status, body := doStatus(t, http.MethodPost, srv.URL+"/api/v1/jobs/"+running.ID+"/cancel", token, nil); status != http.StatusConflict {
		t.Fatalf("cancelling a running job must conflict: %d %v", status, body)
	}
	if status, body := doStatus(t, http.MethodPost, srv.URL+"/api/v1/jobs/"+running.ID+"/retry", token, nil); status != http.StatusConflict {
		t.Fatalf("retrying a running job must conflict: %d %v", status, body)
	}
	succeeded, _ := st.EnqueueJob(&store.Job{
		Type: "account.reconcile", ResourceType: "account", ResourceID: accountID,
		Payload: map[string]any{"account_id": accountID}, State: "succeeded",
	})
	if status, _ := doStatus(t, http.MethodPost, srv.URL+"/api/v1/jobs/"+succeeded.ID+"/cancel", token, nil); status != http.StatusConflict {
		t.Fatalf("cancelling a succeeded job must conflict: %d", status)
	}
}

func TestJobCancelHidesForeignJobsBeforeCheckingWriteCapability(t *testing.T) {
	srv, st, admin := directorFixture(t)
	pid := firstPackageID(t, srv, admin)
	reseller := post(t, srv.URL+"/api/v1/resellers", admin, map[string]any{
		"name": "Read Only Jobs", "username": "readonly-jobs", "password": "ResellerPass!2026",
	})
	resellerID := reseller["id"].(string)
	ownAccount := seedAccount(t, srv, admin, "cancelown", "cancel-own.test", pid)
	doJSON(t, http.MethodPatch, srv.URL+"/api/v1/accounts/"+ownAccount, admin, map[string]any{"reseller_id": resellerID})
	foreignAccount := seedAccount(t, srv, admin, "cancelforeign", "cancel-foreign.test", pid)
	rs := st.GetReseller(resellerID)
	rs.PrivilegeMask = []string{rbac.AccountsRead}
	st.PutReseller(rs)
	resellerToken := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "readonly-jobs", "password": "ResellerPass!2026",
	})["token"].(string)

	enqueue := func(accountID string) *store.Job {
		job, err := st.EnqueueJob(&store.Job{
			Type: "account.reconcile", ResourceType: "account", ResourceID: accountID,
			Payload: map[string]any{"account_id": accountID}, State: "queued",
		})
		if err != nil {
			t.Fatal(err)
		}
		return job
	}
	ownJob := enqueue(ownAccount)
	foreignJob := enqueue(foreignAccount)
	if status, _ := doStatus(t, http.MethodPost, srv.URL+"/api/v1/jobs/"+ownJob.ID+"/cancel", resellerToken, nil); status != http.StatusForbidden {
		t.Fatalf("own job cancellation without accounts.modify: %d", status)
	}
	if status, _ := doStatus(t, http.MethodPost, srv.URL+"/api/v1/jobs/"+foreignJob.ID+"/cancel", resellerToken, nil); status != http.StatusNotFound {
		t.Fatalf("foreign job cancellation should be hidden before capability check: %d", status)
	}
	if st.GetJob(ownJob.ID).State != "queued" || st.GetJob(foreignJob.ID).State != "queued" {
		t.Fatal("unauthorized cancellation changed job state")
	}
	st.PutUsage(&store.Usage{AccountID: ownAccount, DiskBytes: 10})
	st.PutUsage(&store.Usage{AccountID: foreignAccount, DiskBytes: 20})
	accountList := get(t, srv.URL+"/api/v1/accounts", resellerToken)
	if accountList["total"].(float64) != 1 {
		t.Fatalf("reseller account visibility: %v", accountList)
	}
	usage := accountList["usage"].(map[string]any)
	if usage[ownAccount] == nil || usage[foreignAccount] != nil {
		t.Fatalf("account list leaked foreign usage: %v", usage)
	}
}

func TestAuditFiltersAndPaging(t *testing.T) {
	srv, st, token := directorFixture(t)
	pid := firstPackageID(t, srv, token)
	aid := seedAccount(t, srv, token, "audited", "audited.test", pid)
	post(t, srv.URL+"/api/v1/accounts/"+aid+"/suspend", token, nil)

	all := get(t, srv.URL+"/api/v1/audit-events", token)
	if all["total"].(float64) < 2 {
		t.Fatalf("expected recorded events: %v", all)
	}

	suspends := get(t, srv.URL+"/api/v1/audit-events?action=account.suspend", token)
	items := suspends["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["action"] != "account.suspend" {
		t.Fatalf("action filter: %v", suspends)
	}

	byResource := get(t, srv.URL+"/api/v1/audit-events?resource_type=account", token)
	for _, raw := range byResource["items"].([]any) {
		if raw.(map[string]any)["resource_type"] != "account" {
			t.Fatalf("resource filter leaked: %v", raw)
		}
	}

	st.AppendAudit(store.AuditEvent{ID: id.New(), Action: "account.terminate", ResourceType: "account", Success: false})
	failures := get(t, srv.URL+"/api/v1/audit-events?success=false", token)
	if failures["total"].(float64) != 1 {
		t.Fatalf("success filter: %v", failures)
	}

	page := get(t, srv.URL+"/api/v1/audit-events?limit=1&offset=0", token)
	if len(page["items"].([]any)) != 1 {
		t.Fatalf("limit ignored: %v", page)
	}
	next := get(t, srv.URL+"/api/v1/audit-events?limit=1&offset=1", token)
	if page["items"].([]any)[0].(map[string]any)["id"] == next["items"].([]any)[0].(map[string]any)["id"] {
		t.Fatal("offset returned the same event")
	}

	none := get(t, srv.URL+"/api/v1/audit-events?q=no-such-thing-anywhere", token)
	if none["total"].(float64) != 0 {
		t.Fatalf("search filter: %v", none)
	}
	if status, _ := doStatus(t, http.MethodGet, srv.URL+"/api/v1/audit-events?success=maybe", token, nil); status != http.StatusBadRequest {
		t.Fatalf("invalid success filter status: %d", status)
	}
	occurred := time.Now().UTC().Add(-time.Minute)
	actorID := st.UserByUsername("admin").ID
	st.AppendAudit(store.AuditEvent{
		ID: id.New(), OccurredAt: occurred, ActorType: "user", ActorID: actorID,
		AccountID: aid, Action: "account.filter-target", ResourceType: "account",
		RequestID: id.New(), Success: true,
	})
	scoped := get(t, srv.URL+"/api/v1/audit-events?account_id="+aid+
		"&actor_id="+actorID+"&since="+occurred.Add(-time.Second).Format(time.RFC3339)+
		"&until="+occurred.Add(time.Second).Format(time.RFC3339), token)
	if scoped["total"].(float64) != 1 {
		t.Fatalf("account, actor, and time filters: %v", scoped)
	}
	for _, invalid := range []string{
		"account_id=not-an-id", "since=not-a-time", "limit=0", "offset=-1",
	} {
		if status, _ := doStatus(t, http.MethodGet, srv.URL+"/api/v1/audit-events?"+invalid, token, nil); status != http.StatusBadRequest {
			t.Fatalf("invalid audit filter %q status: %d", invalid, status)
		}
	}
}

func TestAuditDefaultPreservesDirectorHistory(t *testing.T) {
	srv, st, token := directorFixture(t)
	for index := 0; index < 60; index++ {
		st.AppendAudit(store.AuditEvent{
			ID: id.New(), Action: "account.inspect", ResourceType: "account",
			Success: true,
		})
	}

	result := get(t, srv.URL+"/api/v1/audit-events", token)
	if total := result["total"].(float64); total < 60 {
		t.Fatalf("audit total = %v, want at least 60", total)
	}
	if items := result["items"].([]any); len(items) < 60 {
		t.Fatalf("default audit page truncated to %d events", len(items))
	}
}

func TestServerZoneIndexAndAccountUsageColumns(t *testing.T) {
	srv, st, token := directorFixture(t)
	pid := firstPackageID(t, srv, token)
	aid := seedAccount(t, srv, token, "zoned", "zoned.test", pid)

	dom := st.ListDomains(aid)[0]
	zone := &store.DNSZone{ID: id.New(), AccountID: aid, DomainID: dom.ID, Name: dom.ASCII, Provider: "powerdns"}
	st.PutZone(zone)
	st.PutRecord(&store.DNSRecord{ID: id.New(), ZoneID: zone.ID, Name: "www", Type: "A", Content: "203.0.113.10", TTL: 300})

	zones := get(t, srv.URL+"/api/v1/server/dns/zones", token)["items"].([]any)
	if len(zones) != 1 {
		t.Fatalf("zone index: %v", zones)
	}
	got := zones[0].(map[string]any)
	if got["account_username"] != "zoned" || got["records"].(float64) != 1 {
		t.Fatalf("zone index should join the owning account and record count: %v", got)
	}

	// The List Accounts table reads disk and transfer from this response
	// instead of one usage request per row.
	pkg := st.GetPackage(pid)
	st.PutUsage(&store.Usage{AccountID: aid, DiskBytes: pkg.DiskBytes + 1, BandwidthBytes: 10})
	accounts := get(t, srv.URL+"/api/v1/accounts", token)
	usage := accounts["usage"].(map[string]any)
	if usage[aid].(map[string]any)["disk_bytes"].(float64) != float64(pkg.DiskBytes+1) {
		t.Fatalf("usage not returned with accounts: %v", usage)
	}

	over := get(t, srv.URL+"/api/v1/accounts?over_quota=true", token)
	if over["total"].(float64) != 1 {
		t.Fatalf("over_quota filter: %v", over)
	}

	st.PutUsage(&store.Usage{AccountID: aid, DiskBytes: 1, BandwidthBytes: 1})
	clear := get(t, srv.URL+"/api/v1/accounts?over_quota=true", token)
	if clear["total"].(float64) != 0 {
		t.Fatalf("account under quota must drop out of the filter: %v", clear)
	}

	byPackage := get(t, srv.URL+"/api/v1/accounts?package_id="+pid, token)
	if byPackage["total"].(float64) != 1 {
		t.Fatalf("package filter: %v", byPackage)
	}
	if empty := get(t, srv.URL+"/api/v1/accounts?package_id="+id.New(), token); empty["total"].(float64) != 0 {
		t.Fatalf("unknown package must match nothing: %v", empty)
	}
}

func TestForcePasswordChange(t *testing.T) {
	srv, st, token := directorFixture(t)
	pid := firstPackageID(t, srv, token)
	aid := seedAccount(t, srv, token, "rotated", "rotated.test", pid)
	owner := st.GetAccount(aid).OwnerUserID
	before := st.UserByID(owner).PasswordHash

	out := post(t, srv.URL+"/api/v1/accounts/"+aid+"/password", token, map[string]any{
		"password": "RotatedPass!2026", "must_change_password": true,
	})
	if out["operation_id"] == "" {
		t.Fatalf("password change should queue a reconcile: %v", out)
	}

	after := st.UserByID(owner)
	if after.PasswordHash == before {
		t.Fatal("password hash unchanged")
	}
	if !after.MustChangePassword {
		t.Fatal("must_change_password not recorded")
	}
	if !auth.VerifyPassword(after.PasswordHash, "RotatedPass!2026") {
		t.Fatal("new password does not verify")
	}

	if status, body := doStatus(t, http.MethodPost, srv.URL+"/api/v1/accounts/"+aid+"/password", token, map[string]any{"password": ""}); status != http.StatusBadRequest {
		t.Fatalf("empty password must be refused: %d %v", status, body)
	}
}

func doJSON(t *testing.T, method, url, token string, body any) map[string]any {
	t.Helper()
	status, out := doStatus(t, method, url, token, body)
	if status >= 400 {
		t.Fatalf("%s %s: %d %v", method, url, status, out)
	}
	return out
}

func doStatus(t *testing.T, method, url, token string, body any) (int, map[string]any) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, url, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}
