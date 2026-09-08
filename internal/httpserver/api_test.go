package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/auth"
	"github.com/hosting-panel/panel/internal/id"
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
	if res.StatusCode != 200 || !bytes.Contains(body, []byte("Hosting Panel Control API")) {
		t.Fatalf("%d %s", res.StatusCode, body[:min(len(body), 200)])
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
