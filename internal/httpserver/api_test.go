package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/auth"
	"github.com/hosting-panel/panel/internal/id"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/rbac"
	"github.com/hosting-panel/panel/internal/store"
)

func TestAccountProvisionFlow(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()

	login := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})
	token := login["token"].(string)
	pkgs := get(t, srv.URL+"/api/v1/packages", token)
	items := pkgs["items"].([]any)
	pkg := items[0].(map[string]any)
	created := post(t, srv.URL+"/api/v1/accounts", token, map[string]string{
		"username": "acme42", "primary_domain": "acme.test", "package_id": pkg["id"].(string),
		"owner_email": "owner@acme.test", "owner_password": "TenantPass!2026",
	})
	if created["status"] != "provisioning" {
		t.Fatalf("%v", created)
	}
	if _, ok := created["operation_id"].(string); !ok {
		t.Fatal("missing operation")
	}
	aid := created["resource_id"].(string)
	mig := post(t, srv.URL+"/api/v1/accounts/"+aid+"/migrate", token, map[string]string{
		"username": "moved42", "domain": "moved.test",
	})
	if mig["resource_id"] == aid || mig["resource_id"] == "" {
		t.Fatalf("migrate: %v", mig)
	}
	if mig["source_id"] != aid {
		t.Fatalf("source %v", mig)
	}
}

func TestCPanelImportQueuesHomedirCopy(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	login := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})
	token := login["token"].(string)
	out := post(t, srv.URL+"/api/v1/accounts/import/cpanel", token, map[string]string{
		"root": "../../testdata/cpanel-acme42", "username": "acme42",
	})
	if out["source"] != "cpanel" {
		t.Fatalf("%v", out)
	}
	if out["homedir_job"] == "" {
		t.Fatal("expected CopyHomedir job")
	}
	var dump string
	for _, j := range st.ListJobs("queued", 20) {
		if j.Type != "account.reconcile" {
			continue
		}
		switch items := j.Payload["databases"].(type) {
		case []any:
			for _, item := range items {
				m, _ := item.(map[string]any)
				if s, _ := m["dump"].(string); s != "" {
					dump = s
				}
			}
		case []map[string]any:
			for _, m := range items {
				if s, _ := m["dump"].(string); s != "" {
					dump = s
				}
			}
		}
	}
	if dump == "" {
		t.Fatal("expected mysql dump path on reconcile job")
	}
}

func TestCPanelImportRequiresServerScope(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()

	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})["token"].(string)
	post(t, srv.URL+"/api/v1/resellers", admin, map[string]string{
		"name": "Importer", "username": "cpanel-importer", "password": "ResellerPass!2026",
	})
	reseller := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "cpanel-importer", "password": "ResellerPass!2026",
	})["token"].(string)

	code, _ := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/accounts/import/cpanel", reseller, map[string]string{
		"root": "../../testdata/cpanel-acme42", "username": "acme42",
	}, nil)
	if code != http.StatusForbidden {
		t.Fatalf("reseller cPanel import status %d", code)
	}
}

func TestNativeImportRequiresServerScope(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()

	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})["token"].(string)
	post(t, srv.URL+"/api/v1/resellers", admin, map[string]string{
		"name": "Native Importer", "username": "native-importer", "password": "ResellerPass!2026",
	})
	reseller := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "native-importer", "password": "ResellerPass!2026",
	})["token"].(string)

	code, _ := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/accounts/import", reseller, map[string]any{}, nil)
	if code != http.StatusForbidden {
		t.Fatalf("reseller native import status %d", code)
	}

	pkgID := st.ListPackages()[0].ID
	source := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "native-source", "primary_domain": "native-source.test", "package_id": pkgID,
		"owner_email": "owner@native-source.test", "owner_password": "TenantPass!2026",
	})
	exported := get(t, srv.URL+"/api/v1/accounts/"+source["resource_id"].(string)+"/export", admin)
	code, imported := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/accounts/import?username=native-copy&domain=native-copy.test", admin, exported, nil)
	if code != http.StatusAccepted || imported["resource_id"] == "" {
		t.Fatalf("server native import: %d %v", code, imported)
	}
}

func post(t *testing.T, url, token string, body any) map[string]any {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
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
	if res.StatusCode >= 400 {
		t.Fatalf("%s %d %v", url, res.StatusCode, out)
	}
	return out
}

func TestResellerCannotSeeForeignAccounts(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()

	adminTok := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})["token"].(string)
	pkgs := get(t, srv.URL+"/api/v1/packages", adminTok)
	pkg := pkgs["items"].([]any)[0].(map[string]any)["id"].(string)
	direct := post(t, srv.URL+"/api/v1/accounts", adminTok, map[string]string{
		"username": "direct1", "primary_domain": "direct.test", "package_id": pkg,
		"owner_email": "o@direct.test", "owner_password": "TenantPass!2026",
	})
	directID := direct["resource_id"].(string)

	rs := post(t, srv.URL+"/api/v1/resellers", adminTok, map[string]string{
		"name": "Northwind", "username": "northwind", "password": "ResellerPass!2026",
	})
	if rs["name"] != "Northwind" {
		t.Fatalf("reseller: %v", rs)
	}
	rsTok := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "northwind", "password": "ResellerPass!2026"})["token"].(string)
	listed := get(t, srv.URL+"/api/v1/accounts", rsTok)
	if items := listed["items"].([]any); len(items) != 0 {
		t.Fatalf("reseller saw foreign accounts: %v", items)
	}
	code := statusOf(t, http.MethodGet, srv.URL+"/api/v1/accounts/"+directID, rsTok, nil)
	if code != 403 {
		t.Fatalf("expected 403 for foreign account, got %d", code)
	}
	mine := post(t, srv.URL+"/api/v1/accounts", rsTok, map[string]string{
		"username": "nwcust", "primary_domain": "nwcust.test", "package_id": pkg,
		"owner_email": "ops@nwcust.test", "owner_password": "TenantPass!2026",
	})
	if mine["status"] != "provisioning" {
		t.Fatalf("%v", mine)
	}
	listed = get(t, srv.URL+"/api/v1/accounts", rsTok)
	items := listed["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["username"] != "nwcust" {
		t.Fatalf("reseller list: %v", listed)
	}
	if items[0].(map[string]any)["reseller_id"] != rs["id"] {
		t.Fatalf("account not attached to reseller: %v", items[0])
	}
	if code := statusOf(t, http.MethodPost, srv.URL+"/api/v1/accounts/"+directID+"/suspend", rsTok, map[string]string{}); code != 403 {
		t.Fatalf("foreign suspend %d", code)
	}
	if code := statusOf(t, http.MethodPost, srv.URL+"/api/v1/accounts/"+directID+"/terminate", rsTok, map[string]string{}); code != 403 {
		t.Fatalf("foreign terminate %d", code)
	}
	if code := statusOf(t, http.MethodGet, srv.URL+"/api/v1/accounts/"+directID+"/export", rsTok, nil); code != 403 {
		t.Fatalf("foreign export %d", code)
	}
	if code := statusOf(t, http.MethodPatch, srv.URL+"/api/v1/accounts/"+directID, rsTok, map[string]string{"package_id": pkg}); code != 403 {
		t.Fatalf("foreign modify %d", code)
	}
	if code := statusOf(t, http.MethodGet, srv.URL+"/api/v1/accounts/"+directID+"/usage", rsTok, nil); code != 403 {
		t.Fatalf("foreign usage %d", code)
	}
	bulk := statusOf(t, http.MethodPost, srv.URL+"/api/v1/accounts/bulk/suspend", rsTok, map[string]any{"ids": []string{directID}})
	if bulk != 202 && bulk != 200 {
		t.Fatalf("bulk status %d", bulk)
	}
	if got := st.GetAccount(directID); got == nil || got.Status == "suspended" {
		t.Fatalf("bulk suspend crossed reseller boundary: %+v", got)
	}
	beforeUnsuspend := st.GetAccount(directID)
	bulkUnsuspend := statusOf(t, http.MethodPost, srv.URL+"/api/v1/accounts/bulk/unsuspend", rsTok, map[string]any{"ids": []string{directID}})
	if bulkUnsuspend != 202 && bulkUnsuspend != 200 {
		t.Fatalf("bulk unsuspend status %d", bulkUnsuspend)
	}
	if got := st.GetAccount(directID); got == nil || beforeUnsuspend == nil || got.DesiredRevision != beforeUnsuspend.DesiredRevision {
		t.Fatalf("bulk unsuspend crossed reseller boundary: %+v", got)
	}
	dump := get(t, srv.URL+"/api/v1/accounts/export", rsTok)
	for _, raw := range dump["items"].([]any) {
		if raw.(map[string]any)["username"] == "direct1" {
			t.Fatal("bulk export leaked foreign account")
		}
	}
	secretJob, err := st.EnqueueJob(&store.Job{
		Type: "account.reconcile", ResourceType: "account", ResourceID: directID,
		Payload: map[string]any{"account_id": directID, "linux_password": "SecretPass!2026"},
		State:   "queued",
	})
	if err != nil {
		t.Fatal(err)
	}
	if code := statusOf(t, http.MethodGet, srv.URL+"/api/v1/jobs/"+secretJob.ID, rsTok, nil); code != 404 {
		t.Fatalf("foreign job should 404, got %d", code)
	}
}

func statusOf(t *testing.T, method, url, token string, body any) int {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, url, rdr)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	return res.StatusCode
}

func TestPackageLimitsAndDiskQuota(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	token := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})["token"].(string)
	pkg := post(t, srv.URL+"/api/v1/packages", token, map[string]any{
		"name": "Tiny", "domains": 1, "subdomains": 0, "alias_domains": 0,
		"databases": 1, "database_users": 1, "mailboxes": 1, "mailbox_storage_bytes": 4096,
		"cron_jobs": 1, "application_instances": 1,
		"ftp_users": 1, "disk_bytes": 8,
	})
	created := post(t, srv.URL+"/api/v1/accounts", token, map[string]string{
		"username": "tiny1", "primary_domain": "tiny.test", "package_id": pkg["id"].(string),
		"owner_email": "o@tiny.test", "owner_password": "TenantPass!2026",
	})
	aid := created["resource_id"].(string)
	code, body := postStatus(t, srv.URL+"/api/v1/accounts/"+aid+"/domains", token, map[string]string{
		"fqdn": "addon.tiny.test", "type": "addon",
	})
	if code != 403 {
		t.Fatalf("addon domain: %d %v", code, body)
	}
	if err, _ := body["error"].(map[string]any); err == nil || err["code"] != "PACKAGE_LIMIT" {
		t.Fatalf("code: %v", body)
	}
	code, _ = postStatus(t, srv.URL+"/api/v1/accounts/"+aid+"/databases", token, map[string]string{"name": "one", "engine": "mariadb"})
	if code >= 400 {
		t.Fatalf("first db should succeed: %d", code)
	}
	code, body = postStatus(t, srv.URL+"/api/v1/accounts/"+aid+"/databases", token, map[string]string{"name": "two", "engine": "mariadb"})
	if code != 403 {
		t.Fatalf("second db: %d %v", code, body)
	}
	code, body = postStatus(t, srv.URL+"/api/v1/accounts/"+aid+"/files", token, map[string]string{
		"path": "/public_html/big.txt", "content": "0123456789",
	})
	if code != 403 {
		t.Fatalf("disk quota: %d %v", code, body)
	}
	code, body = postStatus(t, srv.URL+"/api/v1/accounts/"+aid+"/ftp", token, map[string]string{
		"username": "tinyftp", "password": "FtpPass!2026",
	})
	if code >= 400 {
		t.Fatalf("first ftp should succeed: %d %v", code, body)
	}
	code, body = postStatus(t, srv.URL+"/api/v1/accounts/"+aid+"/ftp", token, map[string]string{
		"username": "tinyftp2", "password": "FtpPass!2026",
	})
	if code != 403 {
		t.Fatalf("second ftp: %d %v", code, body)
	}
	code, _ = postStatus(t, srv.URL+"/api/v1/accounts/"+aid+"/websites", token, map[string]string{
		"domain_id": "missing", "runtime": "php",
	})
	if code >= 400 {
		t.Fatalf("first website should succeed: %d", code)
	}
	code, body = postStatus(t, srv.URL+"/api/v1/accounts/"+aid+"/websites", token, map[string]string{
		"domain_id": "missing", "runtime": "php",
	})
	if code != 403 {
		t.Fatalf("second website: %d %v", code, body)
	}
}

func TestRebootRequiresConfirm(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	token := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})["token"].(string)
	code, _ := postStatus(t, srv.URL+"/api/v1/server/reboot", token, map[string]string{"confirm": "no"})
	if code != 400 {
		t.Fatalf("confirm: %d", code)
	}
	code, body := postStatus(t, srv.URL+"/api/v1/server/reboot", token, map[string]string{"confirm": "REBOOT"})
	if code != 202 {
		t.Fatalf("reboot: %d %v", code, body)
	}
}

func postStatus(t *testing.T, url, token string, body any) (int, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
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

func TestCapabilityRBACAndDNSDelete(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	hash, err := auth.HashPassword("AuditPass!2026")
	if err != nil {
		t.Fatal(err)
	}
	st.PutUser(&store.User{
		ID: id.New(), Username: "auditor", Email: "audit@localhost", PasswordHash: hash,
		DisplayName: "Auditor", Status: "active", Roles: []string{"auditor"},
	})
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})["token"].(string)
	me := get(t, srv.URL+"/api/v1/me", admin)
	caps := me["actor"].(map[string]any)["capabilities"].(map[string]any)
	if caps["accounts.terminate"] != true || caps["security.audit.read"] != true {
		t.Fatalf("admin caps: %v", caps)
	}
	aud := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "auditor", "password": "AuditPass!2026"})["token"].(string)
	audMe := get(t, srv.URL+"/api/v1/me", aud)
	audCaps := audMe["actor"].(map[string]any)["capabilities"].(map[string]any)
	if audCaps["accounts.create"] == true || audCaps["packages.write"] == true {
		t.Fatalf("auditor must not write: %v", audCaps)
	}
	if statusOf(t, http.MethodPost, srv.URL+"/api/v1/packages", aud, map[string]string{"name": "Nope"}) != 403 {
		t.Fatal("auditor created package")
	}
	pkg := get(t, srv.URL+"/api/v1/packages", admin)["items"].([]any)[0].(map[string]any)["id"].(string)
	acc := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "rbac1", "primary_domain": "rbac.test", "package_id": pkg,
		"owner_email": "o@rbac.test", "owner_password": "TenantPass!2026",
	})
	aid := acc["resource_id"].(string)
	z := &store.DNSZone{ID: id.New(), AccountID: aid, Name: "rbac.test", Provider: "powerdns"}
	st.PutZone(z)
	rec := &store.DNSRecord{ID: id.New(), ZoneID: z.ID, Name: "www", Type: "A", Content: "203.0.113.10", TTL: 300}
	st.PutRecord(rec)
	if statusOf(t, http.MethodDelete, srv.URL+"/api/v1/accounts/"+aid+"/dns/zones/"+z.ID+"/records/"+rec.ID, aud, nil) != 403 {
		t.Fatal("auditor deleted DNS")
	}
	code, body := delStatus(t, srv.URL+"/api/v1/accounts/"+aid+"/dns/zones/"+z.ID+"/records/"+rec.ID, admin)
	if code != 202 {
		t.Fatalf("delete %d %v", code, body)
	}
	if len(st.ListRecords(z.ID)) != 0 {
		t.Fatal("record remains")
	}
}

func delStatus(t *testing.T, url, token string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func TestSSHKeyValidation(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})["token"].(string)
	pkg := get(t, srv.URL+"/api/v1/packages", admin)["items"].([]any)[0].(map[string]any)["id"].(string)
	acc := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "sshacct1", "primary_domain": "sshacc.test", "package_id": pkg,
		"owner_email": "o@sshacc.test", "owner_password": "TenantPass!2026",
	})
	aid := acc["resource_id"].(string)
	if statusOf(t, http.MethodPost, srv.URL+"/api/v1/accounts/"+aid+"/ssh-keys", admin, map[string]any{"public_key": "not-a-key"}) != 400 {
		t.Fatal("accepted junk key")
	}
	created := post(t, srv.URL+"/api/v1/accounts/"+aid+"/ssh-keys", admin, map[string]any{
		"public_key": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJustATestKeyNotRealAAAAAAAAAAA=",
		"comment":    "laptop",
	})
	if created["label"] != "laptop" {
		t.Fatalf("label from comment: %v", created)
	}
	kid := created["id"].(string)
	if statusOf(t, http.MethodDelete, srv.URL+"/api/v1/accounts/"+aid+"/ssh-keys/"+kid, admin, nil) != 200 {
		t.Fatal("delete ssh key")
	}
	if len(st.ListSSH(aid)) != 0 {
		t.Fatal("ssh key remained")
	}
}

func TestWordPressInstallAPI(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})["token"].(string)
	pkg := get(t, srv.URL+"/api/v1/packages", admin)["items"].([]any)[0].(map[string]any)["id"].(string)
	acc := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "wpacct1", "primary_domain": "wpsite.test", "package_id": pkg,
		"owner_email": "o@wpsite.test", "owner_password": "TenantPass!2026",
	})
	aid := acc["resource_id"].(string)
	site := &store.Website{ID: id.New(), AccountID: aid, DocumentRoot: "/home/wpacct1/public_html", Runtime: "php", Enabled: true}
	st.PutWebsite(site)
	if statusOf(t, http.MethodPost, srv.URL+"/api/v1/accounts/"+aid+"/wordpress", admin, map[string]any{
		"website_id": site.ID, "title": "WP", "admin_user": "wpadmin",
		"admin_password": "WpAdmin!2026", "admin_email": "o@wpsite.test",
	}) != 202 {
		t.Fatal("wordpress install")
	}
	apps := st.ListApps(aid)
	if len(apps) != 1 || apps[0].Runtime != "wordpress" {
		t.Fatalf("%v", apps)
	}
	if statusOf(t, http.MethodDelete, srv.URL+"/api/v1/accounts/"+aid+"/applications/"+apps[0].ID, admin, nil) != 200 {
		t.Fatal("delete application")
	}
	if len(st.ListApps(aid)) != 0 {
		t.Fatal("application remained")
	}
}

func TestCreateDBRejectsUnknownEngine(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})["token"].(string)
	pkg := get(t, srv.URL+"/api/v1/packages", admin)["items"].([]any)[0].(map[string]any)["id"].(string)
	acc := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "pgacct1", "primary_domain": "pgacct.test", "package_id": pkg,
		"owner_email": "o@pgacct.test", "owner_password": "TenantPass!2026",
	})
	aid := acc["resource_id"].(string)
	if statusOf(t, http.MethodPost, srv.URL+"/api/v1/accounts/"+aid+"/databases", admin, map[string]any{
		"name": "x", "engine": "oracle",
	}) != 400 {
		t.Fatal("expected unknown engine 400")
	}
	if statusOf(t, http.MethodPost, srv.URL+"/api/v1/accounts/"+aid+"/databases", admin, map[string]any{
		"name": "app", "engine": "postgres",
	}) != 202 {
		t.Fatal("postgres database")
	}
	found := false
	for _, d := range st.ListDBs(aid) {
		if d.Engine == "postgres" && strings.HasSuffix(d.Name, "_app") {
			found = true
		}
	}
	if !found {
		t.Fatalf("%v", st.ListDBs(aid))
	}
}

func TestCronAndTokenDelete(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})["token"].(string)
	pkg := get(t, srv.URL+"/api/v1/packages", admin)["items"].([]any)[0].(map[string]any)["id"].(string)
	acc := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "crondel1", "primary_domain": "crondel.test", "package_id": pkg,
		"owner_email": "o@crondel.test", "owner_password": "TenantPass!2026",
	})
	aid := acc["resource_id"].(string)
	created := post(t, srv.URL+"/api/v1/accounts/"+aid+"/cron", admin, map[string]any{
		"schedule": "0 * * * *", "command": "true",
	})
	cron := created["cron"].(map[string]any)
	cid := cron["id"].(string)
	if statusOf(t, http.MethodDelete, srv.URL+"/api/v1/accounts/"+aid+"/cron/"+cid, admin, nil) != 202 {
		t.Fatal("delete cron")
	}
	if len(st.ListCrons(aid)) != 0 {
		t.Fatal("cron remained")
	}
	tok := post(t, srv.URL+"/api/v1/accounts/"+aid+"/api-tokens", admin, map[string]any{
		"name": "ci", "scope": "account", "account_id": aid,
	})
	tid := tok["id"].(string)
	if statusOf(t, http.MethodDelete, srv.URL+"/api/v1/accounts/"+aid+"/api-tokens/"+tid, admin, nil) != 200 {
		t.Fatal("delete token")
	}
	if len(st.ListTokens(st.UserByUsername("admin").ID)) != 0 {
		t.Fatal("token remained")
	}
}

func TestDNSSECAndCatchallAPI(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})["token"].(string)
	pkg := get(t, srv.URL+"/api/v1/packages", admin)["items"].([]any)[0].(map[string]any)["id"].(string)
	acc := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "dnssec1", "primary_domain": "signed.test", "package_id": pkg,
		"owner_email": "o@signed.test", "owner_password": "TenantPass!2026",
	})
	aid := acc["resource_id"].(string)
	z := &store.DNSZone{ID: id.New(), AccountID: aid, Name: "signed.test", Provider: "powerdns"}
	st.PutZone(z)
	if statusOf(t, http.MethodPost, srv.URL+"/api/v1/accounts/"+aid+"/dns/zones/"+z.ID+"/dnssec", admin, map[string]any{"enabled": true}) != 202 {
		t.Fatal("dnssec enable")
	}
	if got := st.GetZone(z.ID); got == nil || !got.DNSSECEnabled {
		t.Fatal("zone not signed in store")
	}
	ds := get(t, srv.URL+"/api/v1/accounts/"+aid+"/dns/zones/"+z.ID+"/ds", admin)
	if _, ok := ds["items"]; !ok {
		t.Fatalf("%v", ds)
	}
	dom := &store.Domain{ID: id.New(), AccountID: aid, ASCII: "signed.test"}
	st.PutDomain(dom)
	md := &store.MailDomain{ID: id.New(), AccountID: aid, DomainID: dom.ID, CatchallPolicy: "reject", Status: "active"}
	st.PutMailDomain(md)
	if statusOf(t, http.MethodPatch, srv.URL+"/api/v1/accounts/"+aid+"/mail/domains/"+md.ID, admin, map[string]any{"catchall_policy": "info"}) != 202 {
		t.Fatal("catchall")
	}
	if st.ListMailDomains(aid)[0].CatchallPolicy != "info" {
		t.Fatal(st.ListMailDomains(aid)[0].CatchallPolicy)
	}
}

func TestMailAliasAndMailboxDelete(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})["token"].(string)
	pkg := get(t, srv.URL+"/api/v1/packages", admin)["items"].([]any)[0].(map[string]any)["id"].(string)
	acc := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "mailal1", "primary_domain": "aliasbox.test", "package_id": pkg,
		"owner_email": "o@aliasbox.test", "owner_password": "TenantPass!2026",
	})
	aid := acc["resource_id"].(string)
	dom := &store.Domain{ID: id.New(), AccountID: aid, ASCII: "aliasbox.test"}
	st.PutDomain(dom)
	md := &store.MailDomain{ID: id.New(), AccountID: aid, DomainID: dom.ID, Status: "active"}
	st.PutMailDomain(md)
	mb := &store.Mailbox{ID: id.New(), AccountID: aid, DomainID: md.ID, LocalPart: "info", Status: "active"}
	st.PutMailbox(mb)
	created := post(t, srv.URL+"/api/v1/accounts/"+aid+"/mail/aliases", admin, map[string]any{
		"domain_id": md.ID, "address": "sales", "destination": "info",
	})
	if created["alias"] == nil {
		t.Fatalf("%v", created)
	}
	if statusOf(t, http.MethodDelete, srv.URL+"/api/v1/accounts/"+aid+"/mail/mailboxes/"+mb.ID, admin, nil) != 409 {
		t.Fatal("expected mailbox in use")
	}
	alid := created["alias"].(map[string]any)["id"].(string)
	if statusOf(t, http.MethodDelete, srv.URL+"/api/v1/accounts/"+aid+"/mail/aliases/"+alid, admin, nil) != 202 {
		t.Fatal("delete alias")
	}
	if statusOf(t, http.MethodDelete, srv.URL+"/api/v1/accounts/"+aid+"/mail/mailboxes/"+mb.ID, admin, nil) != 202 {
		t.Fatal("delete mailbox")
	}
	if st.GetMailbox(mb.ID) != nil {
		t.Fatal("mailbox remained")
	}
}

func TestDeleteDomainPrimaryConflict(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})["token"].(string)
	pkg := get(t, srv.URL+"/api/v1/packages", admin)["items"].([]any)[0].(map[string]any)["id"].(string)
	acc := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "domlab1", "primary_domain": "domlab.test", "package_id": pkg,
		"owner_email": "o@domlab.test", "owner_password": "TenantPass!2026",
	})
	aid := acc["resource_id"].(string)
	primary := &store.Domain{ID: id.New(), AccountID: aid, ASCII: "domlab.test", Type: "primary"}
	addon := &store.Domain{ID: id.New(), AccountID: aid, ASCII: "shop.domlab.test", Type: "addon"}
	st.PutDomain(primary)
	st.PutDomain(addon)
	if statusOf(t, http.MethodDelete, srv.URL+"/api/v1/accounts/"+aid+"/domains/"+primary.ID, admin, nil) != 409 {
		t.Fatal("expected primary domain delete conflict")
	}
	if statusOf(t, http.MethodDelete, srv.URL+"/api/v1/accounts/"+aid+"/domains/"+addon.ID, admin, nil) != 202 {
		t.Fatal("addon domain delete")
	}
}

func TestDeleteWebsitePrimaryConflict(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})["token"].(string)
	pkg := get(t, srv.URL+"/api/v1/packages", admin)["items"].([]any)[0].(map[string]any)["id"].(string)
	acc := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "weblab1", "primary_domain": "weblab.test", "package_id": pkg,
		"owner_email": "o@weblab.test", "owner_password": "TenantPass!2026",
	})
	aid := acc["resource_id"].(string)
	primary := &store.Domain{ID: id.New(), AccountID: aid, ASCII: "weblab.test", Type: "primary"}
	addon := &store.Domain{ID: id.New(), AccountID: aid, ASCII: "blog.weblab.test", Type: "subdomain"}
	st.PutDomain(primary)
	st.PutDomain(addon)
	psite := &store.Website{ID: id.New(), AccountID: aid, DomainID: primary.ID, DocumentRoot: "/home/weblab1/public_html", Runtime: "php", Enabled: true}
	asite := &store.Website{ID: id.New(), AccountID: aid, DomainID: addon.ID, DocumentRoot: "/home/weblab1/blog.weblab.test", Runtime: "php", Enabled: true}
	st.PutWebsite(psite)
	st.PutWebsite(asite)
	if statusOf(t, http.MethodDelete, srv.URL+"/api/v1/accounts/"+aid+"/websites/"+psite.ID, admin, nil) != 409 {
		t.Fatal("expected primary website delete conflict")
	}
	if st.GetWebsite(psite.ID) == nil {
		t.Fatal("primary website removed")
	}
	if statusOf(t, http.MethodDelete, srv.URL+"/api/v1/accounts/"+aid+"/websites/"+asite.ID, admin, nil) != 202 {
		t.Fatal("addon website delete")
	}
}

func TestDeleteDatabaseAndCertOwnership(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})["token"].(string)
	pkg := get(t, srv.URL+"/api/v1/packages", admin)["items"].([]any)[0].(map[string]any)["id"].(string)
	acc := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "dblab1", "primary_domain": "dblab.test", "package_id": pkg,
		"owner_email": "o@dblab.test", "owner_password": "TenantPass!2026",
	})
	aid := acc["resource_id"].(string)
	created := post(t, srv.URL+"/api/v1/accounts/"+aid+"/databases", admin, map[string]any{"name": "scratch", "engine": "mariadb"})
	db := created["database"].(map[string]any)
	if db["name"] != "dblab1_scratch" {
		t.Fatalf("%v", created)
	}
	did := db["id"].(string)
	if statusOf(t, http.MethodDelete, srv.URL+"/api/v1/accounts/"+aid+"/databases/"+did, admin, nil) != 202 {
		t.Fatal("delete database")
	}
	if statusOf(t, http.MethodPost, srv.URL+"/api/v1/accounts/"+aid+"/certificates", admin, map[string]any{"hostname": "evil.example"}) != 400 {
		t.Fatal("foreign cert hostname")
	}
	if statusOf(t, http.MethodPost, srv.URL+"/api/v1/accounts/"+aid+"/certificates", admin, map[string]any{"hostname": "dblab.test"}) != 202 {
		t.Fatal("owned cert hostname")
	}
	first := post(t, srv.URL+"/api/v1/accounts/"+aid+"/certificates", admin, map[string]any{"hostname": "dblab.test"})
	second := post(t, srv.URL+"/api/v1/accounts/"+aid+"/certificates", admin, map[string]any{"hostname": "dblab.test"})
	id1 := first["certificate"].(map[string]any)["id"]
	id2 := second["certificate"].(map[string]any)["id"]
	if id1 != id2 {
		t.Fatalf("cert rows diverged %v %v", id1, id2)
	}
	if n := len(st.ListCerts(aid)); n != 1 {
		t.Fatalf("expected one cert, got %d", n)
	}
}

func TestCreateWebsiteReusesDomainRow(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})["token"].(string)
	pkg := get(t, srv.URL+"/api/v1/packages", admin)["items"].([]any)[0].(map[string]any)["id"].(string)
	acc := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "onesite1", "primary_domain": "onesite.test", "package_id": pkg,
		"owner_email": "o@onesite.test", "owner_password": "TenantPass!2026",
	})
	aid := acc["resource_id"].(string)
	d := &store.Domain{ID: id.New(), AccountID: aid, FQDN: "app.onesite.test", ASCII: "app.onesite.test", Type: "addon"}
	st.PutDomain(d)
	site := &store.Website{ID: id.New(), AccountID: aid, DomainID: d.ID, Runtime: "php", DocumentRoot: "/home/onesite1/app.onesite.test"}
	st.PutWebsite(site)
	out := post(t, srv.URL+"/api/v1/accounts/"+aid+"/websites", admin, map[string]any{
		"domain_id": d.ID, "runtime": "node", "document_root": "/home/onesite1/app.onesite.test",
	})
	got := out["website"].(map[string]any)
	if got["id"] != site.ID {
		t.Fatalf("expected existing website, got %v", out)
	}
	if got["runtime"] != "node" {
		t.Fatalf("runtime %v", got["runtime"])
	}
	if len(st.ListWebsites(aid)) != 1 {
		t.Fatalf("duplicate websites: %d", len(st.ListWebsites(aid)))
	}
}

func TestOpenAPIServesYAML(t *testing.T) {
	st := store.NewMemory()
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	res, err := http.Get(srv.URL + "/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || !bytes.Contains(body, []byte("Kelmor Control Plane API")) {
		t.Fatalf("%d %s", res.StatusCode, body[:min(len(body), 200)])
	}
	if bytes.Contains(body, []byte("Hosting Panel")) || bytes.Contains(body, []byte("Server Portal")) {
		t.Fatalf("legacy chrome in openapi.yaml: %s", body[:min(len(body), 200)])
	}
	paths := map[string]bool{}
	for _, line := range strings.Split(string(body), "\n") {
		if !strings.HasPrefix(line, "  /") || !strings.HasSuffix(line, ":") {
			continue
		}
		path := strings.TrimSpace(strings.TrimSuffix(line, ":"))
		if paths[path] {
			t.Fatalf("duplicate OpenAPI path key %q", path)
		}
		paths[path] = true
	}
	if !bytes.Contains(body, []byte("must_change_password:")) || bytes.Contains(body, []byte("force_change:")) {
		t.Fatal("account password schema must use must_change_password")
	}
	if !bytes.Contains(body, []byte("/auth/complete-password-change:")) {
		t.Fatal("OpenAPI is missing forced password completion")
	}
	if !bytes.Contains(body, []byte("password: { type: string, minLength: 12, writeOnly: true }")) {
		t.Fatal("account password minimum must be 12")
	}
}

func TestProductHTMLUsesKelmorChrome(t *testing.T) {
	st := store.NewMemory()
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	for _, path := range []string{"/", "/openapi", "/api/v1/openapi"} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(path, err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("%s %d", path, res.StatusCode)
		}
		if bytes.Contains(body, []byte("Hosting Panel")) || bytes.Contains(body, []byte("Server Portal")) || bytes.Contains(body, []byte("Account Portal")) {
			t.Fatalf("%s still has legacy chrome: %s", path, body)
		}
		if !bytes.Contains(body, []byte("Kelmor")) {
			t.Fatalf("%s missing Kelmor: %s", path, body)
		}
	}
	res, err := http.Get(srv.URL + "/openapi")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !bytes.Contains(body, []byte("Kelmor Control Plane API")) || !bytes.Contains(body, []byte("<title>Kelmor Control Plane API</title>")) {
		t.Fatalf("openapi ui title: %s", body)
	}
}

func TestRequestCertSkipsFreshAndReusesInflight(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	token := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})["token"].(string)
	pkgs := get(t, srv.URL+"/api/v1/packages", token)
	pkg := pkgs["items"].([]any)[0].(map[string]any)["id"].(string)
	created := post(t, srv.URL+"/api/v1/accounts", token, map[string]string{
		"username": "certfresh", "primary_domain": "certfresh.test", "package_id": pkg,
		"owner_email": "o@certfresh.test", "owner_password": "TenantPass!2026",
	})
	aid := created["resource_id"].(string)
	far := time.Now().Add(80 * 24 * time.Hour)
	st.PutCert(&store.Certificate{ID: "c-fresh", AccountID: aid, Hostname: "certfresh.test", Kind: "domain", Status: "active", NotAfter: &far})
	fresh := post(t, srv.URL+"/api/v1/accounts/"+aid+"/certificates", token, map[string]string{"hostname": "certfresh.test"})
	if fresh["operation_id"] != nil {
		t.Fatalf("fresh cert queued: %v", fresh)
	}
	if cert, _ := fresh["certificate"].(map[string]any); cert == nil || cert["id"] != "c-fresh" {
		t.Fatalf("fresh %v", fresh)
	}
	soon := time.Now().Add(5 * 24 * time.Hour)
	st.PutCert(&store.Certificate{ID: "c-soon", AccountID: aid, Hostname: "certfresh.test", Kind: "domain", Status: "active", NotAfter: &soon})
	first := post(t, srv.URL+"/api/v1/accounts/"+aid+"/certificates", token, map[string]string{"hostname": "certfresh.test"})
	op, _ := first["operation_id"].(string)
	if op == "" {
		t.Fatalf("expected renew job: %v", first)
	}
	again := post(t, srv.URL+"/api/v1/accounts/"+aid+"/certificates", token, map[string]string{"hostname": "certfresh.test"})
	if again["operation_id"] != op {
		t.Fatalf("inflight %v vs %v", again["operation_id"], op)
	}
}

func TestDefaultServicesListsControlPlane(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})["token"].(string)
	got := get(t, srv.URL+"/api/v1/server/services", admin)
	items, _ := got["services"].([]any)
	names := map[string]bool{}
	for _, raw := range items {
		m, _ := raw.(map[string]any)
		if n, _ := m["name"].(string); n != "" {
			names[n] = true
		}
	}
	for _, need := range []string{"nginx", "vsftpd", "panel-api", "panel-worker", "panel-agent", "postgresql", "mariadb"} {
		if !names[need] {
			t.Fatalf("missing service %s in %v", need, names)
		}
	}
}

func TestPackageUpdateAndSafeDelete(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	hash, err := auth.HashPassword("AuditPass!2026")
	if err != nil {
		t.Fatal(err)
	}
	st.PutUser(&store.User{
		ID: id.New(), Username: "package-auditor", Email: "audit@localhost", PasswordHash: hash,
		DisplayName: "Auditor", Status: "active", Roles: []string{"auditor"},
	})
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})["token"].(string)
	auditor := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "package-auditor", "password": "AuditPass!2026"})["token"].(string)
	featureSetID := st.ListFeatureSets()[0].ID

	original := &store.Package{ID: id.New(), ResellerID: id.New(), Name: "Original", FeatureSetID: "features-old"}
	st.PutPackage(original)
	updated := store.Package{
		ID: "cannot-replace-id", ResellerID: id.New(), Name: "Updated", FeatureSetID: featureSetID,
		DiskBytes: 101, BandwidthBytesMonthly: 102, Domains: 3, Subdomains: 4, AliasDomains: 5,
		Databases: 6, DatabaseUsers: 7, Mailboxes: 8, MailboxStorageBytes: 109, FTPUsers: 10,
		CronJobs: 11, ApplicationInstances: 12, BackupRetentionDays: 13, CPUPercent: 14,
		MemoryBytes: 115, ProcessLimit: 16, IOWeight: 17, IOPS: 18, ConcurrentWebRequests: 19,
		EmailDailyLimit: 20,
	}
	negative := updated
	negative.DiskBytes = -1
	if code, _ := requestJSONStatus(t, http.MethodPatch, srv.URL+"/api/v1/packages/"+original.ID, admin, negative, nil); code != http.StatusBadRequest {
		t.Fatalf("negative package update status %d", code)
	}
	if got := st.GetPackage(original.ID); !reflect.DeepEqual(got, original) {
		t.Fatalf("negative update changed package: %+v", got)
	}
	if code, _ := requestJSONStatus(t, http.MethodPatch, srv.URL+"/api/v1/packages/"+original.ID, auditor, updated, nil); code != http.StatusForbidden {
		t.Fatalf("auditor package update status %d", code)
	}
	code, body := requestJSONStatus(t, http.MethodPatch, srv.URL+"/api/v1/packages/"+original.ID, admin, updated, nil)
	if code != http.StatusOK {
		t.Fatalf("package update: %d %v", code, body)
	}
	expected := updated
	expected.ID = original.ID
	expected.ResellerID = original.ResellerID
	if got := st.GetPackage(original.ID); !reflect.DeepEqual(got, &expected) {
		t.Fatalf("package mismatch\n got: %+v\nwant: %+v", got, &expected)
	}

	st.PutAccount(&store.Account{ID: id.New(), Username: "assigned", PackageID: original.ID})
	if code, _ := requestJSONStatus(t, http.MethodDelete, srv.URL+"/api/v1/packages/"+original.ID, admin, nil, nil); code != http.StatusConflict {
		t.Fatalf("assigned package delete status %d", code)
	}
	unassigned := &store.Package{ID: id.New(), Name: "Disposable", FeatureSetID: featureSetID}
	st.PutPackage(unassigned)
	if code, _ := requestJSONStatus(t, http.MethodDelete, srv.URL+"/api/v1/packages/"+unassigned.ID, auditor, nil, nil); code != http.StatusForbidden {
		t.Fatalf("auditor package delete status %d", code)
	}
	if code, body := requestJSONStatus(t, http.MethodDelete, srv.URL+"/api/v1/packages/"+unassigned.ID, admin, nil, nil); code != http.StatusOK {
		t.Fatalf("unassigned package delete: %d %v", code, body)
	}
	if st.GetPackage(unassigned.ID) != nil {
		t.Fatal("unassigned package remains")
	}
	foundAudit := false
	for _, event := range st.ListAudit(20) {
		if event.Action == "package.delete" && event.ResourceID == unassigned.ID {
			foundAudit = true
		}
	}
	if !foundAudit {
		t.Fatal("package delete was not audited")
	}
}

func TestPackageCreateAndUpdateRejectUnknownFeatureSet(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})["token"].(string)

	beforeCreate := len(st.ListPackages())
	if code, _ := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/packages", admin, map[string]any{
		"name": "Unknown features", "feature_set_id": "missing-feature-set",
	}, nil); code != http.StatusBadRequest {
		t.Fatalf("unknown feature set create status %d", code)
	}
	if got := len(st.ListPackages()); got != beforeCreate {
		t.Fatalf("unknown feature set create persisted package: %d -> %d", beforeCreate, got)
	}

	original := st.ListPackages()[0]
	update := original
	update.Name = "Must not persist"
	update.FeatureSetID = "missing-feature-set"
	if code, _ := requestJSONStatus(t, http.MethodPatch, srv.URL+"/api/v1/packages/"+original.ID, admin, update, nil); code != http.StatusBadRequest {
		t.Fatalf("unknown feature set update status %d", code)
	}
	if got := st.GetPackage(original.ID); !reflect.DeepEqual(got, &original) {
		t.Fatalf("unknown feature set update changed package: %+v", got)
	}

	created := post(t, srv.URL+"/api/v1/packages", admin, map[string]any{"name": "Default features"})
	defaultID, _ := created["feature_set_id"].(string)
	if defaultID == "" || defaultID != st.ListFeatureSets()[0].ID {
		t.Fatalf("default feature set assignment: %v", created)
	}

	if code, _ := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/packages", admin, map[string]any{
		"name": "Unknown reseller owner", "reseller_id": id.New(),
	}, nil); code != http.StatusBadRequest {
		t.Fatalf("unknown reseller package owner status %d", code)
	}
}

func TestResellerUpdateRequiresServerScopeAndPersistsEditableFields(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})["token"].(string)
	created := post(t, srv.URL+"/api/v1/resellers", admin, map[string]any{
		"name": "Original", "username": "scope-reseller", "password": "ResellerPass!2026",
	})
	resellerID := created["id"].(string)
	ownerID := created["user_id"].(string)

	scopedPlain := "hp_live_reseller_update_scope"
	st.PutToken(&store.APIToken{
		ID: id.New(), UserID: st.UserByUsername("admin").ID, Name: "scoped",
		TokenHash: auth.HashToken(scopedPlain), Scope: "account", AccountID: id.New(),
		Capabilities: []string{"resellers.modify"},
	})
	update := map[string]any{
		"id": id.New(), "user_id": id.New(), "name": "Updated Reseller", "brand_name": "Updated Brand",
		"privilege_mask": []string{"accounts.create", "packages.write"},
		"nameservers":    []string{"ns1.updated.test", "ns2.updated.test"}, "status": "suspended",
	}
	if code, _ := requestJSONStatus(t, http.MethodPatch, srv.URL+"/api/v1/resellers/"+resellerID, scopedPlain, update, nil); code != http.StatusForbidden {
		t.Fatalf("account-scoped reseller update status %d", code)
	}
	code, body := requestJSONStatus(t, http.MethodPatch, srv.URL+"/api/v1/resellers/"+resellerID, admin, update, nil)
	if code != http.StatusOK {
		t.Fatalf("reseller update: %d %v", code, body)
	}
	got := st.GetReseller(resellerID)
	if got == nil || got.ID != resellerID || got.UserID != ownerID || got.Name != "Updated Reseller" ||
		got.BrandName != "Updated Brand" || got.Status != "suspended" ||
		!reflect.DeepEqual(got.PrivilegeMask, []string{"accounts.create", "packages.write"}) ||
		!reflect.DeepEqual(got.Nameservers, []string{"ns1.updated.test", "ns2.updated.test"}) {
		t.Fatalf("reseller mismatch: %+v", got)
	}
}

func TestAccountAPITokensStayWithinURLAccount(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})["token"].(string)
	pkgID := get(t, srv.URL+"/api/v1/packages", admin)["items"].([]any)[0].(map[string]any)["id"].(string)
	createAccount := func(username, domain string) string {
		t.Helper()
		created := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
			"username": username, "primary_domain": domain, "package_id": pkgID,
			"owner_email": username + "@" + domain, "owner_password": "TenantPass!2026",
		})
		return created["resource_id"].(string)
	}
	accountID := createAccount("tokenone", "token-one.test")
	foreignAccountID := createAccount("tokentwo", "token-two.test")
	ownerToken := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "tokenone", "password": "TenantPass!2026",
	})["token"].(string)

	for _, capability := range []string{rbac.ServerSettingsWrite, rbac.ServerFirewallWrite, "unknown.capability"} {
		code, body := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/accounts/"+accountID+"/api-tokens", ownerToken, map[string]any{
			"name": "unsafe", "capabilities": []string{capability},
		}, nil)
		if code != http.StatusBadRequest {
			t.Fatalf("unsafe capability %q status %d: %v", capability, code, body)
		}
	}
	if code, _ := requestJSONStatus(t, http.MethodGet, srv.URL+"/api/v1/accounts/"+foreignAccountID+"/api-tokens", ownerToken, nil, nil); code != http.StatusForbidden {
		t.Fatalf("foreign token list status %d", code)
	}
	if code, _ := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/accounts/"+foreignAccountID+"/api-tokens", ownerToken, map[string]any{
		"name": "foreign", "capabilities": []string{rbac.DNSRead},
	}, nil); code != http.StatusForbidden {
		t.Fatalf("foreign token create status %d", code)
	}

	code, created := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/accounts/"+accountID+"/api-tokens", ownerToken, map[string]any{
		"name": "dns-reader", "scope": "server", "account_id": foreignAccountID,
		"capabilities": []string{rbac.DNSRead},
	}, nil)
	if code != http.StatusCreated {
		t.Fatalf("safe token create: %d %v", code, created)
	}
	apiToken := created["token"].(string)
	tokenID := created["id"].(string)
	stored := st.GetToken(tokenID)
	if stored == nil || stored.Scope != "account" || stored.AccountID != accountID ||
		!reflect.DeepEqual(stored.Capabilities, []string{rbac.DNSRead}) {
		t.Fatalf("stored account token: %+v", stored)
	}
	st.PutToken(&store.APIToken{
		ID: id.New(), UserID: stored.UserID, Name: "foreign", Scope: "account",
		AccountID: foreignAccountID, Capabilities: []string{rbac.DNSRead},
	})
	listed := get(t, srv.URL+"/api/v1/accounts/"+accountID+"/api-tokens", ownerToken)
	items := listed["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["id"] != tokenID {
		t.Fatalf("account token list leaked another account: %v", listed)
	}
	if code := statusOf(t, http.MethodGet, srv.URL+"/api/v1/accounts/"+accountID+"/dns/zones", apiToken, nil); code != http.StatusOK {
		t.Fatalf("own-account token status %d", code)
	}
	if code := statusOf(t, http.MethodGet, srv.URL+"/api/v1/accounts/"+foreignAccountID+"/dns/zones", apiToken, nil); code != http.StatusForbidden {
		t.Fatalf("cross-account token status %d", code)
	}
	me := get(t, srv.URL+"/api/v1/me", apiToken)["actor"].(map[string]any)
	if me["is_server_scope"] != false || !reflect.DeepEqual(me["account_ids"], []any{accountID}) {
		t.Fatalf("account token actor scope: %v", me)
	}
	if code, _ := requestJSONStatus(t, http.MethodDelete, srv.URL+"/api/v1/accounts/"+foreignAccountID+"/api-tokens/"+tokenID, ownerToken, nil, nil); code != http.StatusForbidden {
		t.Fatalf("foreign token delete status %d", code)
	}

	owner := st.UserByUsername("tokenone")
	owner.Status = "inactive"
	st.PutUser(owner)
	if code := statusOf(t, http.MethodGet, srv.URL+"/api/v1/me", apiToken, nil); code != http.StatusUnauthorized {
		t.Fatalf("inactive user's API token status %d", code)
	}
}

func TestDNSRecordRoutesRejectForeignZone(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})["token"].(string)
	pkgID := get(t, srv.URL+"/api/v1/packages", admin)["items"].([]any)[0].(map[string]any)["id"].(string)
	first := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "dnsfirst", "primary_domain": "dns-first.test", "package_id": pkgID,
		"owner_email": "owner@dns-first.test", "owner_password": "TenantPass!2026",
	})["resource_id"].(string)
	second := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "dnssecond", "primary_domain": "dns-second.test", "package_id": pkgID,
		"owner_email": "owner@dns-second.test", "owner_password": "TenantPass!2026",
	})["resource_id"].(string)
	zone := &store.DNSZone{ID: id.New(), AccountID: second, Name: "dns-second.test", Provider: "powerdns"}
	st.PutZone(zone)
	st.PutRecord(&store.DNSRecord{ID: id.New(), ZoneID: zone.ID, Name: "www", Type: "A", Content: "203.0.113.10", TTL: 300})

	url := srv.URL + "/api/v1/accounts/" + first + "/dns/zones/" + zone.ID + "/records"
	if code, _ := requestJSONStatus(t, http.MethodGet, url, admin, nil, nil); code != http.StatusNotFound {
		t.Fatalf("foreign zone read status %d", code)
	}
	before := len(st.ListRecords(zone.ID))
	if code, _ := requestJSONStatus(t, http.MethodPost, url, admin, map[string]any{
		"name": "new", "type": "A", "content": "203.0.113.11", "ttl": 300,
	}, nil); code != http.StatusNotFound {
		t.Fatalf("foreign zone create status %d", code)
	}
	if got := len(st.ListRecords(zone.ID)); got != before {
		t.Fatalf("foreign zone record count changed from %d to %d", before, got)
	}
}

func TestCreateApplicationValidatesAccountInputs(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})["token"].(string)
	pkgID := get(t, srv.URL+"/api/v1/packages", admin)["items"].([]any)[0].(map[string]any)["id"].(string)
	first := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "appfirst", "primary_domain": "app-first.test", "package_id": pkgID,
		"owner_email": "owner@app-first.test", "owner_password": "TenantPass!2026",
	})["resource_id"].(string)
	second := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "appsecond", "primary_domain": "app-second.test", "package_id": pkgID,
		"owner_email": "owner@app-second.test", "owner_password": "TenantPass!2026",
	})["resource_id"].(string)
	ownSite := &store.Website{ID: id.New(), AccountID: first, DocumentRoot: "/home/appfirst/public_html"}
	foreignSite := &store.Website{ID: id.New(), AccountID: second, DocumentRoot: "/home/appsecond/public_html"}
	st.PutWebsite(ownSite)
	st.PutWebsite(foreignSite)
	url := srv.URL + "/api/v1/accounts/" + first + "/applications"
	valid := map[string]any{
		"website_id": ownSite.ID, "runtime": "node", "runtime_version": "20",
		"working_directory": "/home/appfirst/app", "start_command": "npm start",
	}
	invalid := []map[string]any{
		{"website_id": foreignSite.ID, "runtime": "node", "working_directory": "/home/appfirst/app", "start_command": "npm start"},
		{"website_id": ownSite.ID, "runtime": "php", "working_directory": "/home/appfirst/app", "start_command": "php index.php"},
		{"website_id": ownSite.ID, "runtime": "node", "working_directory": "/home/appsecond/app", "start_command": "npm start"},
		{"website_id": ownSite.ID, "runtime": "node", "working_directory": "/home/appfirst/app/../escape", "start_command": "npm start"},
		{"website_id": ownSite.ID, "runtime": "node", "working_directory": "/home/appfirst/app\nUser=root", "start_command": "npm start"},
		{"website_id": ownSite.ID, "runtime": "node\nUser=root", "working_directory": "/home/appfirst/app", "start_command": "npm start"},
		{"website_id": ownSite.ID, "runtime": "node", "runtime_version": "20\nUser=root", "working_directory": "/home/appfirst/app", "start_command": "npm start"},
		{"website_id": ownSite.ID, "runtime": "node", "working_directory": "/home/appfirst/app", "start_command": "npm start\nUser=root"},
	}
	for i, body := range invalid {
		if code, response := requestJSONStatus(t, http.MethodPost, url, admin, body, nil); code != http.StatusBadRequest {
			t.Fatalf("invalid application %d status %d: %v", i, code, response)
		}
	}
	if code, body := requestJSONStatus(t, http.MethodPost, url, admin, valid, nil); code != http.StatusAccepted {
		t.Fatalf("valid application: %d %v", code, body)
	}
}

func TestResellerPrivilegeMaskAndStatusAreEnforced(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})["token"].(string)
	created := post(t, srv.URL+"/api/v1/resellers", admin, map[string]any{
		"name": "Restricted", "username": "restricted-reseller", "password": "ResellerPass!2026",
	})
	resellerID := created["id"].(string)
	if !reflect.DeepEqual(created["privilege_mask"], stringsToAny(rbac.RoleCaps["reseller"])) {
		t.Fatalf("new reseller default privileges: %v", created["privilege_mask"])
	}
	resellerToken := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "restricted-reseller", "password": "ResellerPass!2026",
	})["token"].(string)
	legacy := st.GetReseller(resellerID)
	legacy.PrivilegeMask = []string{}
	st.PutReseller(legacy)
	if code := statusOf(t, http.MethodPost, srv.URL+"/api/v1/packages", resellerToken, map[string]any{"name": "legacy-default"}); code != http.StatusForbidden {
		t.Fatalf("legacy empty privilege mask status %d", code)
	}

	for _, update := range []map[string]any{
		{"privilege_mask": []string{rbac.ServerSettingsWrite}},
		{"privilege_mask": []string{"unknown.capability"}},
		{"status": "deleted"},
	} {
		if code, body := requestJSONStatus(t, http.MethodPatch, srv.URL+"/api/v1/resellers/"+resellerID, admin, update, nil); code != http.StatusBadRequest {
			t.Fatalf("unsafe reseller update status %d: %v", code, body)
		}
	}
	if code, body := requestJSONStatus(t, http.MethodPatch, srv.URL+"/api/v1/resellers/"+resellerID, admin, map[string]any{
		"privilege_mask": []string{rbac.AccountsRead}, "status": "active",
	}, nil); code != http.StatusOK {
		t.Fatalf("restrict reseller: %d %v", code, body)
	}
	if code := statusOf(t, http.MethodPost, srv.URL+"/api/v1/packages", resellerToken, map[string]any{"name": "forbidden"}); code != http.StatusForbidden {
		t.Fatalf("restricted reseller package create status %d", code)
	}
	code, emptied := requestJSONStatus(t, http.MethodPatch, srv.URL+"/api/v1/resellers/"+resellerID, admin, map[string]any{
		"privilege_mask": []string{}, "status": "active",
	}, nil)
	if code != http.StatusOK || !reflect.DeepEqual(emptied["privilege_mask"], []any{}) {
		t.Fatalf("empty reseller privilege update: %d %v", code, emptied)
	}
	if code := statusOf(t, http.MethodGet, srv.URL+"/api/v1/accounts", resellerToken, nil); code != http.StatusForbidden {
		t.Fatalf("empty privilege reseller account list status %d", code)
	}
	if code, body := requestJSONStatus(t, http.MethodPatch, srv.URL+"/api/v1/resellers/"+resellerID, admin, map[string]any{
		"status": "suspended",
	}, nil); code != http.StatusOK {
		t.Fatalf("suspend reseller: %d %v", code, body)
	}
	if code := statusOf(t, http.MethodGet, srv.URL+"/api/v1/accounts", resellerToken, nil); code != http.StatusUnauthorized {
		t.Fatalf("suspended reseller existing session status %d", code)
	}
	if code, _ := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "restricted-reseller", "password": "ResellerPass!2026",
	}, nil); code != http.StatusUnauthorized {
		t.Fatalf("suspended reseller login status %d", code)
	}
}

func stringsToAny(values []string) []any {
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}

func TestModifyAccountValidatesPackageAndReseller(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})["token"].(string)
	firstReseller := post(t, srv.URL+"/api/v1/resellers", admin, map[string]any{"name": "First"})["id"].(string)
	secondReseller := post(t, srv.URL+"/api/v1/resellers", admin, map[string]any{"name": "Second"})["id"].(string)
	globalPackage := get(t, srv.URL+"/api/v1/packages", admin)["items"].([]any)[0].(map[string]any)["id"].(string)
	ownPackage := &store.Package{ID: id.New(), ResellerID: firstReseller, Name: "First package"}
	foreignPackage := &store.Package{ID: id.New(), ResellerID: secondReseller, Name: "Second package"}
	st.PutPackage(ownPackage)
	st.PutPackage(foreignPackage)
	accountID := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "packageacct", "primary_domain": "package-account.test", "package_id": globalPackage,
		"reseller_id": firstReseller, "owner_email": "owner@package-account.test", "owner_password": "TenantPass!2026",
	})["resource_id"].(string)
	url := srv.URL + "/api/v1/accounts/" + accountID
	if code, _ := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "mismatchacct", "primary_domain": "mismatch-account.test", "package_id": ownPackage.ID,
		"reseller_id": secondReseller, "owner_email": "owner@mismatch-account.test", "owner_password": "TenantPass!2026",
	}, nil); code != http.StatusForbidden {
		t.Fatalf("mismatched private package create status %d", code)
	}
	if code, body := requestJSONStatus(t, http.MethodPatch, url, admin, map[string]any{"package_id": ownPackage.ID}, nil); code != http.StatusAccepted {
		t.Fatalf("own package assignment: %d %v", code, body)
	}
	if code, _ := requestJSONStatus(t, http.MethodPatch, url, admin, map[string]any{"reseller_id": secondReseller}, nil); code != http.StatusForbidden {
		t.Fatalf("reseller-only private package mismatch status %d", code)
	}
	if code, _ := requestJSONStatus(t, http.MethodPatch, url, admin, map[string]any{"package_id": foreignPackage.ID}, nil); code != http.StatusForbidden {
		t.Fatalf("foreign package assignment status %d", code)
	}
	if code, _ := requestJSONStatus(t, http.MethodPatch, url, admin, map[string]any{"package_id": id.New()}, nil); code != http.StatusBadRequest {
		t.Fatalf("unknown package assignment status %d", code)
	}
	if code, _ := requestJSONStatus(t, http.MethodPatch, url, admin, map[string]any{"reseller_id": id.New()}, nil); code != http.StatusBadRequest {
		t.Fatalf("unknown reseller assignment status %d", code)
	}
	if code, body := requestJSONStatus(t, http.MethodPatch, url, admin, map[string]any{
		"package_id": globalPackage, "reseller_id": "",
	}, nil); code != http.StatusAccepted {
		t.Fatalf("clear reseller assignment: %d %v", code, body)
	}
}

func TestCreateAccountRejectsWeakOwnerPassword(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})["token"].(string)
	pkgID := st.ListPackages()[0].ID

	code, body := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "weakowner", "primary_domain": "weak-owner.test", "package_id": pkgID,
		"owner_email": "owner@weak-owner.test", "owner_password": "12345678901",
	}, nil)
	assertAPIErrorCode(t, code, body, http.StatusBadRequest, "VALIDATION")
	if st.AccountByUsername("weakowner") != nil || st.UserByUsername("weakowner") != nil {
		t.Fatal("weak-password account creation persisted state")
	}
}

func TestServerProcessesReportsOnlyTruthfulProcessIdentity(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})["token"].(string)

	items := get(t, srv.URL+"/api/v1/server/processes", admin)["processes"].([]any)
	if len(items) == 0 {
		t.Fatal("expected a live process list")
	}
	self := os.Getpid()
	found := false
	for _, item := range items {
		process := item.(map[string]any)
		if process["pid"] == nil || process["name"] == nil {
			t.Fatalf("process missing identity: %v", process)
		}
		if int(process["pid"].(float64)) == self {
			found = true
		}
	}
	if !found {
		t.Fatalf("current pid %d missing from process list", self)
	}
	code, body := postStatus(t, srv.URL+"/api/v1/server/processes/1/signal", admin, map[string]string{"signal": "TERM"})
	if code != 400 {
		t.Fatalf("pid 1 signal %d %v", code, body)
	}
}

func sortedMapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func TestAccountOwnerPasswordRotation(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})["token"].(string)
	pkg := get(t, srv.URL+"/api/v1/packages", admin)["items"].([]any)[0].(map[string]any)["id"].(string)
	created := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "rotate1", "primary_domain": "rotate.test", "package_id": pkg,
		"owner_email": "o@rotate.test", "owner_password": "OriginalPass!2026",
	})
	accountID := created["resource_id"].(string)
	if code, _ := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/accounts/"+accountID+"/password", admin, map[string]any{
		"password": "12345678901", "must_change_password": true,
	}, nil); code != http.StatusBadRequest {
		t.Fatalf("11-character password status %d", code)
	}
	code, body := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/accounts/"+accountID+"/password", admin, map[string]any{
		"password": "RotatedPass!2026", "must_change_password": true,
	}, map[string]string{"X-Request-ID": id.New()})
	if code != http.StatusAccepted {
		t.Fatalf("password rotation: %d %v", code, body)
	}
	owner := st.UserByID(st.GetAccount(accountID).OwnerUserID)
	if owner == nil || !auth.VerifyPassword(owner.PasswordHash, "RotatedPass!2026") ||
		auth.VerifyPassword(owner.PasswordHash, "OriginalPass!2026") || !owner.MustChangePassword ||
		!reflect.DeepEqual(owner.Roles, []string{"customer_owner"}) {
		t.Fatalf("owner credentials were not rotated: %+v", owner)
	}
	operationID, _ := body["operation_id"].(string)
	queued := st.GetJob(operationID)
	if queued == nil || queued.Type != "account.reconcile" || queued.State != "queued" ||
		queued.Payload["account_id"] != accountID || queued.Payload["linux_password"] != "RotatedPass!2026" ||
		queued.ID == "" || queued.CreatedAt.IsZero() || queued.RunAfter.IsZero() ||
		queued.MaxAttempts != 5 || queued.Logs == nil {
		t.Fatalf("password reconcile job: %+v", queued)
	}
	response, _ := json.Marshal(body)
	if bytes.Contains(response, []byte("RotatedPass!2026")) || bytes.Contains(response, []byte("password_hash")) {
		t.Fatalf("password response leaked credentials: %s", response)
	}
	jobResponse, _ := json.Marshal(get(t, srv.URL+"/api/v1/jobs/"+operationID, admin))
	if bytes.Contains(jobResponse, []byte("RotatedPass!2026")) || bytes.Contains(jobResponse, []byte("linux_password")) {
		t.Fatalf("job response leaked credentials: %s", jobResponse)
	}
	jobListResponse, _ := json.Marshal(get(t, srv.URL+"/api/v1/jobs", admin))
	if bytes.Contains(jobListResponse, []byte("RotatedPass!2026")) || bytes.Contains(jobListResponse, []byte("linux_password")) {
		t.Fatalf("job list response leaked credentials: %s", jobListResponse)
	}
}

func TestFailedJobRetryClonesSafePayloadAndPreservesHistory(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})["token"].(string)
	pkg := get(t, srv.URL+"/api/v1/packages", admin)["items"].([]any)[0].(map[string]any)["id"].(string)
	direct := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "retry1", "primary_domain": "retry.test", "package_id": pkg,
		"owner_email": "o@retry.test", "owner_password": "OriginalPass!2026",
	})
	accountID := direct["resource_id"].(string)
	failed, err := st.EnqueueJob(&store.Job{
		Type: "account.reconcile", ResourceType: "account", ResourceID: accountID,
		Payload: map[string]any{"account_id": accountID, "status": "active", "nested": map[string]any{"safe": "retained"}},
		State:   "failed", Priority: 7, Attempts: 5, MaxAttempts: 5, LastError: "agent failed",
		ActorID: st.UserByUsername("admin").ID, RequestID: id.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	safePublic := get(t, srv.URL+"/api/v1/jobs/"+failed.ID, admin)
	if retryable, ok := safePublic["retryable"].(bool); !ok || !retryable {
		t.Fatalf("safe failed job retryable = %v", safePublic["retryable"])
	}
	before, _ := json.Marshal(st.GetJob(failed.ID))
	requestID := id.New()
	code, body := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/jobs/"+failed.ID+"/retry", admin, nil, map[string]string{"X-Request-ID": requestID})
	if code != http.StatusAccepted {
		t.Fatalf("job retry: %d %v", code, body)
	}
	retryID, _ := body["operation_id"].(string)
	retry := st.GetJob(retryID)
	if retry == nil || retry.ID == failed.ID || retry.Type != failed.Type || retry.ResourceType != failed.ResourceType ||
		retry.ResourceID != failed.ResourceID || retry.State != "queued" || retry.Priority != failed.Priority ||
		retry.ActorID != st.UserByUsername("admin").ID || retry.RequestID != requestID ||
		retry.Payload["account_id"] != accountID || retry.Payload["status"] != "active" || retry.Payload["retry_of"] != failed.ID {
		t.Fatalf("retry job mismatch: %+v", retry)
	}
	payload, _ := json.Marshal(retry.Payload)
	if !bytes.Contains(payload, []byte(`"safe":"retained"`)) {
		t.Fatalf("retry payload lost safe nested data: %s", payload)
	}
	after, _ := json.Marshal(st.GetJob(failed.ID))
	if !bytes.Equal(before, after) {
		t.Fatalf("original job mutated\nbefore: %s\nafter:  %s", before, after)
	}
	if code, _ := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/jobs/"+retry.ID+"/retry", admin, nil, nil); code != http.StatusConflict {
		t.Fatalf("queued job retry status %d", code)
	}
	foundAudit := false
	for _, event := range st.ListAudit(20) {
		if event.Action == "job.retry" && event.ResourceID == failed.ID && event.After["retry_job_id"] == retry.ID {
			foundAudit = true
		}
	}
	if !foundAudit {
		t.Fatal("job retry was not audited")
	}

	for _, secretPayload := range []map[string]any{
		{"account_id": accountID, "linux_password": "MustNotBeCloned!2026"},
		{"account_id": accountID, "steps": []any{map[string]any{"admin_password": "WordPressPass!2026"}}},
	} {
		secretJob, enqueueErr := st.EnqueueJob(&store.Job{
			Type: "account.reconcile", ResourceType: "account", ResourceID: accountID,
			Payload: secretPayload, State: "failed",
		})
		if enqueueErr != nil {
			t.Fatal(enqueueErr)
		}
		sensitivePublic := get(t, srv.URL+"/api/v1/jobs/"+secretJob.ID, admin)
		if retryable, ok := sensitivePublic["retryable"].(bool); !ok || retryable {
			t.Fatalf("sensitive failed job retryable = %v", sensitivePublic["retryable"])
		}
		code, rejected := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/jobs/"+secretJob.ID+"/retry", admin, nil, nil)
		if code != http.StatusConflict {
			t.Fatalf("sensitive retry status %d: %v", code, rejected)
		}
		apiError, _ := rejected["error"].(map[string]any)
		if apiError["code"] != "NOT_RETRYABLE" {
			t.Fatalf("sensitive retry error: %v", rejected)
		}
	}

	reseller := post(t, srv.URL+"/api/v1/resellers", admin, map[string]string{
		"name": "Retry Reseller", "username": "retry-reseller", "password": "ResellerPass!2026",
	})
	if reseller["id"] == "" {
		t.Fatalf("reseller: %v", reseller)
	}
	resellerToken := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "retry-reseller", "password": "ResellerPass!2026",
	})["token"].(string)
	if code, _ := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/jobs/"+failed.ID+"/retry", resellerToken, nil, nil); code != http.StatusNotFound {
		t.Fatalf("inaccessible job retry status %d", code)
	}
}

func TestResellerCanAccessOwnResourceJobsButCannotRetryWithoutWrite(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})["token"].(string)
	pkg := get(t, srv.URL+"/api/v1/packages", admin)["items"].([]any)[0].(map[string]any)["id"].(string)
	post(t, srv.URL+"/api/v1/resellers", admin, map[string]string{
		"name": "Resource Jobs", "username": "resource-jobs", "password": "ResellerPass!2026",
	})
	reseller := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "resource-jobs", "password": "ResellerPass!2026",
	})["token"].(string)
	ownAccount := post(t, srv.URL+"/api/v1/accounts", reseller, map[string]string{
		"username": "ownjobs", "primary_domain": "ownjobs.test", "package_id": pkg,
		"owner_email": "owner@ownjobs.test", "owner_password": "TenantPass!2026",
	})["resource_id"].(string)
	foreignAccount := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "foreignjobs", "primary_domain": "foreignjobs.test", "package_id": pkg,
		"owner_email": "owner@foreignjobs.test", "owner_password": "TenantPass!2026",
	})["resource_id"].(string)

	ownCert := &store.Certificate{ID: id.New(), AccountID: ownAccount, Hostname: "ownjobs.test"}
	foreignCert := &store.Certificate{ID: id.New(), AccountID: foreignAccount, Hostname: "foreignjobs.test"}
	ownWebsite := &store.Website{ID: id.New(), AccountID: ownAccount, DocumentRoot: "/home/ownjobs/public_html"}
	foreignWebsite := &store.Website{ID: id.New(), AccountID: foreignAccount, DocumentRoot: "/home/foreignjobs/public_html"}
	st.PutCert(ownCert)
	st.PutCert(foreignCert)
	st.PutWebsite(ownWebsite)
	st.PutWebsite(foreignWebsite)

	enqueueFailed := func(jobType, resourceType, resourceID string) *store.Job {
		t.Helper()
		job, err := st.EnqueueJob(&store.Job{
			Type: jobType, ResourceType: resourceType, ResourceID: resourceID,
			Payload: map[string]any{"reason": "safe"}, State: "failed", LastError: "agent failed",
		})
		if err != nil {
			t.Fatal(err)
		}
		return job
	}
	ownJobs := []*store.Job{
		enqueueFailed("certificate.provision", "certificate", ownCert.ID),
		enqueueFailed("website.provision", "website", ownWebsite.ID),
	}
	foreignJobs := []*store.Job{
		enqueueFailed("certificate.provision", "certificate", foreignCert.ID),
		enqueueFailed("website.provision", "website", foreignWebsite.ID),
	}

	listed := get(t, srv.URL+"/api/v1/jobs", reseller)["items"].([]any)
	listedIDs := map[string]bool{}
	for _, item := range listed {
		listedIDs[item.(map[string]any)["id"].(string)] = true
	}
	for _, job := range ownJobs {
		if !listedIDs[job.ID] {
			t.Errorf("own %s job missing from list", job.ResourceType)
		}
		if code, _ := requestJSONStatus(t, http.MethodGet, srv.URL+"/api/v1/jobs/"+job.ID, reseller, nil, nil); code != http.StatusOK {
			t.Errorf("own %s job get status %d", job.ResourceType, code)
		}
		if code, body := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/jobs/"+job.ID+"/retry", reseller, nil, nil); code != http.StatusForbidden {
			t.Errorf("own %s job retry without websites.write status %d: %v", job.ResourceType, code, body)
		}
	}
	for _, job := range foreignJobs {
		if listedIDs[job.ID] {
			t.Errorf("foreign %s job present in list", job.ResourceType)
		}
		if code, _ := requestJSONStatus(t, http.MethodGet, srv.URL+"/api/v1/jobs/"+job.ID, reseller, nil, nil); code != http.StatusNotFound {
			t.Errorf("foreign %s job get status %d", job.ResourceType, code)
		}
		if code, _ := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/jobs/"+job.ID+"/retry", reseller, nil, nil); code != http.StatusNotFound {
			t.Errorf("foreign %s job retry status %d", job.ResourceType, code)
		}
	}
}

func TestJobAccountIDResolvesStoredResources(t *testing.T) {
	st := store.NewMemory()
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	accountID := id.New()

	domain := &store.Domain{ID: id.New(), AccountID: accountID}
	website := &store.Website{ID: id.New(), AccountID: accountID}
	application := &store.Application{ID: id.New(), AccountID: accountID}
	database := &store.HostedDatabase{ID: id.New(), AccountID: accountID}
	zone := &store.DNSZone{ID: id.New(), AccountID: accountID}
	mailDomain := &store.MailDomain{ID: id.New(), AccountID: accountID}
	mailbox := &store.Mailbox{ID: id.New(), AccountID: accountID}
	mailAlias := &store.MailAlias{ID: id.New(), AccountID: accountID}
	certificate := &store.Certificate{ID: id.New(), AccountID: accountID}
	backup := &store.BackupRun{ID: id.New(), AccountID: accountID}
	cron := &store.CronJob{ID: id.New(), AccountID: accountID}
	sshKey := &store.SSHKey{ID: id.New(), AccountID: accountID}
	st.PutDomain(domain)
	st.PutWebsite(website)
	st.PutApp(application)
	st.PutDB(database)
	st.PutZone(zone)
	st.PutMailDomain(mailDomain)
	st.PutMailbox(mailbox)
	st.PutMailAlias(mailAlias)
	st.PutCert(certificate)
	st.PutBackup(backup)
	st.PutCron(cron)
	st.PutSSH(sshKey)

	resources := []struct {
		resourceType string
		resourceID   string
	}{
		{"domain", domain.ID},
		{"website", website.ID},
		{"application", application.ID},
		{"database", database.ID},
		{"dns_zone", zone.ID},
		{"mail_domain", mailDomain.ID},
		{"mailbox", mailbox.ID},
		{"mail_alias", mailAlias.ID},
		{"certificate", certificate.ID},
		{"backup", backup.ID},
		{"cron_job", cron.ID},
		{"ssh_key", sshKey.ID},
	}
	for _, resource := range resources {
		t.Run(resource.resourceType, func(t *testing.T) {
			job := &store.Job{ResourceType: resource.resourceType, ResourceID: resource.resourceID}
			if got := api.jobAccountID(job); got != accountID {
				t.Fatalf("job account ID %q, want %q", got, accountID)
			}
		})
	}
	if got := api.jobAccountID(&store.Job{
		ResourceType: "missing", ResourceID: id.New(), Payload: map[string]any{"account_id": accountID},
	}); got != accountID {
		t.Fatalf("payload account ID %q, want %q", got, accountID)
	}
	if got := api.jobAccountID(&store.Job{ResourceType: "website", ResourceID: id.New()}); got != "" {
		t.Fatalf("unproven account ID %q", got)
	}
}

func requestJSONStatus(t *testing.T, method, url, token string, body any, headers map[string]string) (int, map[string]any) {
	t.Helper()
	var requestBody io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		requestBody = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
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

func get(t *testing.T, url, token string) map[string]any {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return out
}

type failingPasswordRotationStore struct {
	store.Store
}

func (s failingPasswordRotationStore) EnqueueJob(*store.Job) (*store.Job, error) {
	return nil, errors.New("queue unavailable")
}

func (s failingPasswordRotationStore) RotatePasswordAndEnqueue(string, string, bool, *store.Job) (*store.Job, error) {
	return nil, errors.New("queue unavailable")
}

type failingAccountMutationStore struct {
	store.Store
}

func (s failingAccountMutationStore) CreateAccountWithJob(*store.User, *store.Account, *store.Domain, []string, *store.Job) (*store.Job, error) {
	return nil, errors.New("queue unavailable")
}

func (s failingAccountMutationStore) UpdateAccountWithJob(*store.Account, *store.Job) (*store.Job, error) {
	return nil, errors.New("queue unavailable")
}

func TestCreateAccountFailureDoesNotPersistStateOrSuccessAudit(t *testing.T) {
	data := store.NewMemory()
	if err := store.SeedDev(data, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(failingAccountMutationStore{Store: data}, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})["token"].(string)
	beforeUsers := data.Stats()["users"]
	beforeDomains := data.Stats()["domains"]

	code, _ := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "atomic-create", "primary_domain": "atomic-create.test",
		"package_id": data.ListPackages()[0].ID, "owner_email": "owner@atomic-create.test",
		"owner_password": "TenantPass!2026",
	}, nil)
	if code != http.StatusInternalServerError {
		t.Fatalf("create failure status %d", code)
	}
	if data.AccountByUsername("atomic-create") != nil || data.UserByUsername("atomic-create") != nil {
		t.Fatal("failed account create persisted account or owner")
	}
	if data.DomainTaken("atomic-create.test") || data.Stats()["domains"] != beforeDomains ||
		data.Stats()["users"] != beforeUsers {
		t.Fatal("failed account create persisted user or domain state")
	}
	if hasSuccessfulAudit(data, "account.create") {
		t.Fatal("failed account create wrote a success audit")
	}
}

func TestModifyAccountFailureDoesNotPersistStateOrSuccessAudit(t *testing.T) {
	data := store.NewMemory()
	if err := store.SeedDev(data, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	original := &store.Account{
		ID: id.New(), Username: "atomic-modify", PrimaryDomain: "before.test",
		PackageID: data.ListPackages()[0].ID, Status: "active", DesiredRevision: 3,
	}
	data.PutAccount(original)
	api := New(failingAccountMutationStore{Store: data}, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})["token"].(string)

	code, _ := requestJSONStatus(t, http.MethodPatch, srv.URL+"/api/v1/accounts/"+original.ID, admin, map[string]any{
		"primary_domain": "after.test", "login_disabled": true,
	}, nil)
	if code != http.StatusInternalServerError {
		t.Fatalf("modify failure status %d", code)
	}
	if got := data.GetAccount(original.ID); !reflect.DeepEqual(got, original) {
		t.Fatalf("failed account modify persisted state: %+v", got)
	}
	if hasSuccessfulAudit(data, "account.modify") {
		t.Fatal("failed account modify wrote a success audit")
	}
}

func TestAccountStatusFailureDoesNotPersistStateOrSuccessAudit(t *testing.T) {
	data := store.NewMemory()
	if err := store.SeedDev(data, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	original := &store.Account{
		ID: id.New(), Username: "atomic-status", PrimaryDomain: "status.test",
		PackageID: data.ListPackages()[0].ID, Status: "active", DesiredRevision: 7,
	}
	data.PutAccount(original)
	api := New(failingAccountMutationStore{Store: data}, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})["token"].(string)

	code, _ := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/accounts/"+original.ID+"/suspend", admin, map[string]any{}, nil)
	if code != http.StatusInternalServerError {
		t.Fatalf("status failure status %d", code)
	}
	if got := data.GetAccount(original.ID); !reflect.DeepEqual(got, original) {
		t.Fatalf("failed account status change persisted state: %+v", got)
	}
	if hasSuccessfulAudit(data, "account.suspend") {
		t.Fatal("failed account status change wrote a success audit")
	}
}

func hasSuccessfulAudit(data store.Store, action string) bool {
	for _, event := range data.ListAudit(100) {
		if event.Action == action && event.Success {
			return true
		}
	}
	return false
}

func TestAccountPasswordRotationFailureDoesNotChangeOwner(t *testing.T) {
	data := store.NewMemory()
	if err := store.SeedDev(data, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	ownerHash, err := auth.HashPassword("OriginalPass!2026")
	if err != nil {
		t.Fatal(err)
	}
	owner := &store.User{
		ID: "owner-atomic", Username: "atomic-owner", PasswordHash: ownerHash,
		Status: "active", MustChangePassword: false, Roles: []string{"customer_owner"},
	}
	account := &store.Account{ID: "account-atomic", OwnerUserID: owner.ID, Username: owner.Username}
	data.PutUser(owner)
	data.PutAccount(account)

	api := New(failingPasswordRotationStore{Store: data}, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})["token"].(string)

	code, _ := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/accounts/"+account.ID+"/password", admin, map[string]any{
		"password": "RotatedPass!2026", "must_change_password": true,
	}, nil)
	if code != http.StatusInternalServerError {
		t.Fatalf("rotation failure status %d", code)
	}
	stored := data.UserByID(owner.ID)
	if stored == nil || stored.PasswordHash != ownerHash || stored.MustChangePassword {
		t.Fatalf("failed rotation changed owner credentials: %+v", stored)
	}
	if jobs := data.ListJobs("", 10); len(jobs) != 0 {
		t.Fatalf("failed rotation queued jobs: %+v", jobs)
	}
}

func TestPasswordChangeRequiredBlocksLoginSessionAndAPIToken(t *testing.T) {
	data := store.NewMemory()
	passwordHash, err := auth.HashPassword("CurrentPass!2026")
	if err != nil {
		t.Fatal(err)
	}
	user := &store.User{
		ID: "forced-user", Username: "forced", PasswordHash: passwordHash,
		Status: "active", MustChangePassword: true, Roles: []string{"customer_owner"},
	}
	data.PutUser(user)
	data.PutAccount(&store.Account{ID: "forced-account", OwnerUserID: user.ID, Username: user.Username})

	sessionToken, sessionHash, err := auth.NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	data.PutSession(&store.Session{
		ID: "forced-session", UserID: user.ID, TokenHash: sessionHash, ExpiresAt: time.Now().Add(time.Hour),
	})
	apiToken := "hp_live_forced_password_change"
	data.PutToken(&store.APIToken{
		ID: "forced-token", UserID: user.ID, TokenHash: auth.HashToken(apiToken),
		Scope: "account", AccountID: "forced-account", Capabilities: []string{rbac.DNSRead},
	})

	api := New(data, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()

	code, body := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": user.Username, "password": "CurrentPass!2026",
	}, nil)
	assertAPIErrorCode(t, code, body, http.StatusForbidden, "PASSWORD_CHANGE_REQUIRED")
	for _, token := range []string{sessionToken, apiToken} {
		code, body = requestJSONStatus(t, http.MethodGet, srv.URL+"/api/v1/me", token, nil, nil)
		assertAPIErrorCode(t, code, body, http.StatusForbidden, "PASSWORD_CHANGE_REQUIRED")
	}
}

func TestCompletePasswordChangeValidatesAndQueuesReconcile(t *testing.T) {
	data := store.NewMemory()
	passwordHash, err := auth.HashPassword("CurrentPass!2026")
	if err != nil {
		t.Fatal(err)
	}
	user := &store.User{
		ID: "complete-user", Username: "complete", PasswordHash: passwordHash,
		Status: "active", MustChangePassword: true, Roles: []string{"customer_owner"},
	}
	account := &store.Account{ID: "complete-account", OwnerUserID: user.ID, Username: user.Username}
	data.PutUser(user)
	data.PutAccount(account)

	api := New(data, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	endpoint := srv.URL + "/api/v1/auth/complete-password-change"

	tests := []struct {
		name        string
		current     string
		next        string
		wantCode    int
		wantAPIcode string
	}{
		{name: "wrong current", current: "WrongPass!2026", next: "Replacement!2026", wantCode: http.StatusUnauthorized, wantAPIcode: "INVALID_CREDENTIALS"},
		{name: "short", current: "CurrentPass!2026", next: "TooShort!1", wantCode: http.StatusBadRequest, wantAPIcode: "VALIDATION"},
		{name: "same", current: "CurrentPass!2026", next: "CurrentPass!2026", wantCode: http.StatusBadRequest, wantAPIcode: "VALIDATION"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, body := requestJSONStatus(t, http.MethodPost, endpoint, "", map[string]string{
				"username": user.Username, "current_password": test.current, "new_password": test.next,
			}, nil)
			assertAPIErrorCode(t, code, body, test.wantCode, test.wantAPIcode)
			stored := data.UserByID(user.ID)
			if stored == nil || stored.PasswordHash != passwordHash || !stored.MustChangePassword {
				t.Fatalf("rejected password change mutated user: %+v", stored)
			}
			if jobs := data.ListJobs("", 10); len(jobs) != 0 {
				t.Fatalf("rejected password change queued jobs: %+v", jobs)
			}
		})
	}

	newPassword := "Replacement!2026"
	code, body := requestJSONStatus(t, http.MethodPost, endpoint, "", map[string]string{
		"username": user.Username, "current_password": "CurrentPass!2026", "new_password": newPassword,
	}, nil)
	if code != http.StatusOK {
		t.Fatalf("complete password change: %d %v", code, body)
	}
	response, _ := json.Marshal(body)
	if bytes.Contains(response, []byte(newPassword)) || bytes.Contains(response, []byte("password")) ||
		bytes.Contains(response, []byte("hash")) {
		t.Fatalf("password change response leaked credentials: %s", response)
	}
	stored := data.UserByID(user.ID)
	if stored == nil || stored.MustChangePassword || !auth.VerifyPassword(stored.PasswordHash, newPassword) ||
		auth.VerifyPassword(stored.PasswordHash, "CurrentPass!2026") {
		t.Fatalf("password change did not update user: %+v", stored)
	}
	jobs := data.ListJobs("queued", 10)
	if len(jobs) != 1 || jobs[0].Type != "account.reconcile" || jobs[0].ResourceID != account.ID ||
		jobs[0].Payload["account_id"] != account.ID || jobs[0].Payload["linux_password"] != newPassword {
		t.Fatalf("password change reconcile job: %+v", jobs)
	}
	foundAudit := false
	for _, event := range data.ListAudit(20) {
		if event.Action == "auth.password.complete" && event.ActorID == user.ID && event.ResourceID == user.ID && event.Success {
			foundAudit = true
		}
	}
	if !foundAudit {
		t.Fatal("completed password change was not audited with the user actor")
	}
}

func TestCompletePasswordChangeRejectsUserWithoutOwnedAccount(t *testing.T) {
	data := store.NewMemory()
	passwordHash, err := auth.HashPassword("CurrentPass!2026")
	if err != nil {
		t.Fatal(err)
	}
	user := &store.User{
		ID: "unowned-user", Username: "unowned", PasswordHash: passwordHash,
		Status: "active", MustChangePassword: true, Roles: []string{"customer_owner"},
	}
	data.PutUser(user)
	api := New(data, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()

	code, _ := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/auth/complete-password-change", "", map[string]string{
		"username": user.Username, "current_password": "CurrentPass!2026", "new_password": "Replacement!2026",
	}, nil)
	if code != http.StatusConflict {
		t.Fatalf("unowned password change status %d", code)
	}
	stored := data.UserByID(user.ID)
	if stored == nil || stored.PasswordHash != passwordHash || !stored.MustChangePassword {
		t.Fatalf("unowned password change mutated user: %+v", stored)
	}
}

func assertAPIErrorCode(t *testing.T, status int, body map[string]any, wantStatus int, wantCode string) {
	t.Helper()
	if status != wantStatus {
		t.Fatalf("status %d, want %d: %v", status, wantStatus, body)
	}
	apiError, _ := body["error"].(map[string]any)
	if apiError["code"] != wantCode {
		t.Fatalf("error code %v, want %s: %v", apiError["code"], wantCode, body)
	}
}
