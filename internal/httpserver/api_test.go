package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/pkg/logging"
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
		"databases": 1, "mailboxes": 1, "cron_jobs": 1, "application_instances": 1,
		"disk_bytes": 8,
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
