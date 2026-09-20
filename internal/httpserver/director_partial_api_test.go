package httpserver

import (
	"net/http"
	"testing"
	"time"

	"github.com/hosting-panel/panel/internal/id"
	"github.com/hosting-panel/panel/internal/store"
	"github.com/pquerna/otp/totp"
)

func TestResetAccountBandwidthQueuesAgentJob(t *testing.T) {
	srv, st, token := directorFixture(t)
	pkg := firstPackageID(t, srv, token)
	aid := seedAccount(t, srv, token, "bwreset", "bwreset.example.test", pkg)
	st.PutUsage(&store.Usage{AccountID: aid, BandwidthBytes: 9 << 30, BandwidthHold: true, CollectedAt: time.Now().UTC()})
	queued := doJSON(t, http.MethodPost, srv.URL+"/api/v1/accounts/"+aid+"/bandwidth/reset", token, map[string]any{})
	if queued["operation_id"] == nil {
		t.Fatalf("reset: %v", queued)
	}
	job := st.GetJob(queued["operation_id"].(string))
	if job == nil || job.Type != "bandwidth.reset" {
		t.Fatalf("job: %+v", job)
	}
	usage := st.GetUsage(aid)
	if usage == nil || usage.BandwidthBytes != 0 || usage.BandwidthHold {
		t.Fatalf("usage not cleared: %+v", usage)
	}
}

func TestResellerListIncludesAccountsPackagesAndUsage(t *testing.T) {
	srv, st, token := directorFixture(t)
	pkg := firstPackageID(t, srv, token)
	aid := seedAccount(t, srv, token, "reslist", "reslist.example.test", pkg)
	created := post(t, srv.URL+"/api/v1/resellers", token, map[string]any{
		"name": "North Desk", "username": "northdesk", "password": "ResellerPass!2026",
		"brand_name": "North Host",
	})
	rid := created["id"].(string)
	owned := post(t, srv.URL+"/api/v1/packages", token, map[string]any{
		"name": "North Desk Plan", "reseller_id": rid,
	})
	acc := st.GetAccount(aid)
	acc.ResellerID = rid
	st.PutAccount(acc)
	st.PutUsage(&store.Usage{AccountID: aid, DiskBytes: 512, BandwidthBytes: 1024})

	listed := get(t, srv.URL+"/api/v1/resellers", token)
	items := listed["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("reseller rows: %v", listed)
	}
	row := items[0].(map[string]any)
	if row["id"] != rid || row["name"] != "North Desk" || row["status"] != "active" {
		t.Fatalf("identity: %v", row)
	}
	if row["accounts"].(float64) != 1 {
		t.Fatalf("accounts: %v", row)
	}
	if row["disk_bytes"].(float64) != 512 || row["bandwidth_bytes"].(float64) != 1024 {
		t.Fatalf("usage: %v", row)
	}
	if row["disk_limit"].(float64) <= 0 || row["bandwidth_limit"].(float64) <= 0 {
		t.Fatalf("limits: %v", row)
	}
	packages, _ := row["packages"].([]any)
	foundOwned := false
	for _, name := range packages {
		if name == owned["name"] {
			foundOwned = true
		}
	}
	if !foundOwned {
		t.Fatalf("packages: %v", row["packages"])
	}
}

func TestRestoreInventoryListsHostBackups(t *testing.T) {
	srv, st, token := directorFixture(t)
	pkg := firstPackageID(t, srv, token)
	aid := seedAccount(t, srv, token, "rstinv", "rstinv.example.test", pkg)
	st.PutBackup(&store.BackupRun{
		ID: "aaaaaaaa-aaaa-7aaa-8aaa-aaaaaaaaaaaa", AccountID: aid,
		Kind: "full", State: "succeeded", Destination: "local", SizeBytes: 2048,
		Checksum: "abc", CreatedAt: time.Now().UTC(),
	})
	st.PutBackup(&store.BackupRun{
		ID: "bbbbbbbb-bbbb-7bbb-8bbb-bbbbbbbbbbbb", AccountID: aid,
		Kind: "full", State: "queued", Destination: "local",
	})
	st.PutBackup(&store.BackupRun{
		ID:   "cccccccc-cccc-7ccc-8ccc-cccccccccccc",
		Kind: "system", State: "succeeded", Destination: "local", SizeBytes: 99,
	})

	listed := get(t, srv.URL+"/api/v1/backups", token)
	items := listed["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("inventory: %v", listed)
	}
	byID := map[string]map[string]any{}
	for _, raw := range items {
		row := raw.(map[string]any)
		byID[row["id"].(string)] = row
	}
	ready := byID["aaaaaaaa-aaaa-7aaa-8aaa-aaaaaaaaaaaa"]
	if ready["restorable"] != true || ready["account_username"] != "rstinv" || ready["scope"] != "account" {
		t.Fatalf("account archive: %v", ready)
	}
	if byID["bbbbbbbb-bbbb-7bbb-8bbb-bbbbbbbbbbbb"]["restorable"] != false {
		t.Fatalf("queued must not be restorable: %v", byID["bbbbbbbb-bbbb-7bbb-8bbb-bbbbbbbbbbbb"])
	}
	system := byID["cccccccc-cccc-7ccc-8ccc-cccccccccccc"]
	if system["scope"] != "system" || system["restorable"] != true {
		t.Fatalf("system archive: %v", system)
	}
}

func TestRestoreInventoryEmptyWhenNone(t *testing.T) {
	srv, _, token := directorFixture(t)
	listed := get(t, srv.URL+"/api/v1/backups", token)
	items, _ := listed["items"].([]any)
	if items == nil || len(items) != 0 {
		t.Fatalf("empty inventory must be []: %v", listed)
	}
}

func TestResellerUsageMetersAndReset(t *testing.T) {
	srv, st, token := directorFixture(t)
	pkg := firstPackageID(t, srv, token)
	aid := seedAccount(t, srv, token, "resuse", "resuse.example.test", pkg)
	reseller := &store.Reseller{ID: id.New(), Name: "North", BrandName: "North Host", Status: "active"}
	st.PutReseller(reseller)
	acc := st.GetAccount(aid)
	acc.ResellerID = reseller.ID
	st.PutAccount(acc)
	st.PutUsage(&store.Usage{AccountID: aid, DiskBytes: 100, BandwidthBytes: 200, BandwidthHold: true})

	listed := get(t, srv.URL+"/api/v1/resellers/usage", token)
	items := listed["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("usage rows: %v", listed)
	}
	row := items[0].(map[string]any)
	if row["id"] != reseller.ID || row["bandwidth_bytes"].(float64) != 200 || row["bandwidth_holds"].(float64) != 1 {
		t.Fatalf("row: %v", row)
	}

	reset := doJSON(t, http.MethodPost, srv.URL+"/api/v1/resellers/"+reseller.ID+"/bandwidth/reset", token, map[string]any{})
	if reset["cleared"].(float64) != 1 {
		t.Fatalf("reset: %v", reset)
	}
	if st.GetUsage(aid).BandwidthBytes != 0 {
		t.Fatal("reseller reset must zero account counters")
	}
}

func TestSkeletonDirectoryCRUD(t *testing.T) {
	srv, _, token := directorFixture(t)
	empty := get(t, srv.URL+"/api/v1/server/skeleton", token)
	if empty["root"] != "/etc/skel" {
		t.Fatalf("list: %v", empty)
	}
	written := doJSON(t, http.MethodPut, srv.URL+"/api/v1/server/skeleton/file", token, map[string]any{
		"path": "public_html/index.html", "content": "<h1>Welcome</h1>\n",
	})
	if written["ok"] != true {
		t.Fatalf("write: %v", written)
	}
	read := get(t, srv.URL+"/api/v1/server/skeleton/file?path=public_html/index.html", token)
	if read["content"] != "<h1>Welcome</h1>\n" {
		t.Fatalf("read: %v", read)
	}
	if status, _ := doStatus(t, http.MethodPut, srv.URL+"/api/v1/server/skeleton/file", token, map[string]any{
		"path": "/etc/passwd", "content": "nope",
	}); status != http.StatusBadRequest {
		t.Fatalf("escape must fail: %d", status)
	}
	deleted := doJSON(t, http.MethodDelete, srv.URL+"/api/v1/server/skeleton/file?path=public_html/index.html", token, nil)
	if deleted["ok"] != true {
		t.Fatalf("delete: %v", deleted)
	}
}

func TestInitialQuotaSetupQueuesHostJob(t *testing.T) {
	srv, st, token := directorFixture(t)
	status := get(t, srv.URL+"/api/v1/server/quota", token)
	if status["host_path"] != "/etc/panel/initial-quota" {
		t.Fatalf("probe: %v", status)
	}
	queued := doJSON(t, http.MethodPost, srv.URL+"/api/v1/server/quota/setup", token, map[string]any{
		"default_disk_mb": 2048, "enforce": true,
	})
	if queued["operation_id"] == nil {
		t.Fatalf("setup: %v", queued)
	}
	job := st.GetJob(queued["operation_id"].(string))
	if job == nil || job.Type != "host.quota.setup" {
		t.Fatalf("job: %+v", job)
	}
}

func TestLinkedNodesAndHostApplySettings(t *testing.T) {
	srv, st, token := directorFixture(t)
	saved := doJSON(t, http.MethodPatch, srv.URL+"/api/v1/server/settings", token, map[string]any{
		"values": map[string]any{
			"linked_nodes":          map[string]string{"nodes": "https://dns-peer.example.test:2087"},
			"configuration_cluster": map[string]string{"peers": "https://mail-peer.example.test:2087"},
			"external_auth":         map[string]string{"provider": "ldap", "ldap_url": "ldap://directory.example.test", "require_external": "off"},
			"two_factor":            map[string]string{"required": "on"},
		},
	})
	if saved["operation_id"] == nil {
		t.Fatalf("host apply must queue: %v", saved)
	}
	job := st.GetJob(saved["operation_id"].(string))
	if job == nil || job.Type != "host.config.apply" {
		t.Fatalf("job: %+v", job)
	}
	nodes := get(t, srv.URL+"/api/v1/server/nodes", token)
	items := nodes["items"].([]any)
	if len(items) < 2 {
		t.Fatalf("nodes: %v", nodes)
	}
}

func TestOperatorTOTPEnrollAndLogin(t *testing.T) {
	srv, st, token := directorFixture(t)
	enrolled := post(t, srv.URL+"/api/v1/auth/totp/enroll", token, map[string]any{})
	secret, _ := enrolled["secret"].(string)
	if secret == "" || enrolled["otpauth_url"] == "" {
		t.Fatalf("enroll: %v", enrolled)
	}
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	confirmed := post(t, srv.URL+"/api/v1/auth/totp/confirm", token, map[string]any{"code": code})
	if confirmed["totp_enabled"] != true {
		t.Fatalf("confirm: %v", confirmed)
	}
	admin := st.UserByUsername("admin")
	if admin == nil || !admin.TOTPEnabled {
		t.Fatal("store must persist totp_enabled")
	}
	if status, body := doStatus(t, http.MethodPost, srv.URL+"/api/v1/auth/login", "", map[string]any{
		"username": "admin", "password": "ChangeMeOnce!2026",
	}); status != http.StatusUnauthorized {
		t.Fatalf("login without totp: %d %v", status, body)
	}
	code, err = totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	logged := post(t, srv.URL+"/api/v1/auth/login", "", map[string]any{
		"username": "admin", "password": "ChangeMeOnce!2026", "totp_code": code,
	})
	if logged["token"] == "" {
		t.Fatalf("login with totp: %v", logged)
	}
}

func TestLDAPRequiredLoginBindsThroughAgent(t *testing.T) {
	srv, _, token := directorFixture(t)
	doJSON(t, http.MethodPatch, srv.URL+"/api/v1/server/settings", token, map[string]any{
		"values": map[string]any{
			"external_auth": map[string]string{
				"provider": "ldap", "ldap_url": "ldap://directory.example.test",
				"ldap_user_dn":     "uid={username},ou=people,dc=kelmor,dc=test",
				"require_external": "on",
			},
		},
	})
	logged := post(t, srv.URL+"/api/v1/auth/login", "", map[string]any{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})
	if logged["token"] == "" {
		t.Fatalf("ldap login: %v", logged)
	}
}

func TestSkeletonRejectsRootDelete(t *testing.T) {
	srv, _, token := directorFixture(t)
	if status, _ := doStatus(t, http.MethodDelete, srv.URL+"/api/v1/server/skeleton/file?path=", token, nil); status != http.StatusBadRequest {
		t.Fatalf("empty path: %d", status)
	}
}

func TestQuotaSetupPersistsDirectorPolicy(t *testing.T) {
	srv, _, token := directorFixture(t)
	doJSON(t, http.MethodPost, srv.URL+"/api/v1/server/quota/setup", token, map[string]any{
		"default_disk_mb": 512, "enforce": true,
	})
	settings := get(t, srv.URL+"/api/v1/server/settings", token)
	values, _ := settings["values"].(map[string]any)
	quota, _ := values["initial_quota"].(map[string]any)
	if quota["default_disk_mb"] != "512" {
		t.Fatalf("settings: %v", settings)
	}
}
