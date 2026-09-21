package httpserver

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/id"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/store"
)

func TestListIPMigrationShowsAccountsOnSourceOrEmpty(t *testing.T) {
	srv, st, token := directorFixture(t)
	pkg := firstPackageID(t, srv, token)
	moved := seedAccount(t, srv, token, "moveuser", "move.example.test", pkg)
	shared := seedAccount(t, srv, token, "shareuser", "share.example.test", pkg)
	acc := st.GetAccount(moved)
	acc.IPAddress = "203.0.113.40"
	st.PutAccount(acc)

	listed := get(t, srv.URL+"/api/v1/accounts/ip-migration?from_ip=203.0.113.40", token)
	items, _ := listed["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected the dedicated-IP tenant, got %v", listed)
	}
	row := items[0].(map[string]any)
	if row["username"] != "moveuser" || row["id"] != moved {
		t.Fatalf("wrong tenant: %v", row)
	}

	empty := get(t, srv.URL+"/api/v1/accounts/ip-migration?from_ip=198.51.100.9", token)
	if got, _ := empty["items"].([]any); len(got) != 0 {
		t.Fatalf("unknown IP must be honest empty: %v", empty)
	}
	if !strings.Contains(empty["note"].(string), "Shared or unset") {
		t.Fatalf("empty note: %v", empty)
	}

	unset := get(t, srv.URL+"/api/v1/accounts/ip-migration?from_ip=", token)
	foundShared := false
	for _, raw := range unset["items"].([]any) {
		if raw.(map[string]any)["id"] == shared {
			foundShared = true
		}
		if raw.(map[string]any)["id"] == moved {
			t.Fatal("dedicated-IP account must not appear on the unset/shared list")
		}
	}
	if !foundShared {
		t.Fatalf("unset IP list should include the shared tenant: %v", unset)
	}
}

func TestListMailNotifyReturnsOwnerRecipients(t *testing.T) {
	srv, _, token := directorFixture(t)
	pkg := firstPackageID(t, srv, token)
	seedAccount(t, srv, token, "orbixtar", "orbixtar.dpdns.org", pkg)

	listed := get(t, srv.URL+"/api/v1/mail/notify?audience=owners", token)
	if listed["path"] != "POST /mail/notify" {
		t.Fatalf("preview must name the real send path: %v", listed)
	}
	items, _ := listed["items"].([]any)
	found := false
	for _, raw := range items {
		row := raw.(map[string]any)
		if row["username"] == "orbixtar" && row["email"] == "owner@orbixtar.dpdns.org" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected orbixtar owner email, got %v", listed)
	}
}

func TestNginxLogsListHomeAndVhostPathsAndDownload(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	handler := New(st, logging.New("test"), &operations.Host{Root: root})
	srv := httptest.NewServer(handler.Handler())
	t.Cleanup(srv.Close)
	token := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})["token"].(string)
	pkg := firstPackageID(t, srv, token)
	aid := seedAccount(t, srv, token, "loguser", "logs.example.test", pkg)
	acc := st.GetAccount(aid)
	siteID := id.New()
	st.PutWebsite(&store.Website{
		ID: siteID, AccountID: aid, DomainID: st.ListDomains(aid)[0].ID,
		DocumentRoot: acc.HomePath + "/public_html", Runtime: "php", Enabled: true,
	})

	listed := get(t, srv.URL+"/api/v1/accounts/"+aid+"/nginx-logs", token)
	items, _ := listed["items"].([]any)
	if len(items) < 4 {
		t.Fatalf("expected home + vhost logs, got %v", listed)
	}
	var homeAccess, vhostAccess string
	for _, raw := range items {
		row := raw.(map[string]any)
		path, _ := row["path"].(string)
		if path == "/home/loguser/logs/access.log" {
			homeAccess = path
			if row["present"] != false {
				t.Fatalf("missing home log must report present=false: %v", row)
			}
		}
		if path == "/var/log/nginx/"+siteID+".access.log" {
			vhostAccess = path
		}
	}
	if homeAccess == "" || vhostAccess == "" {
		t.Fatalf("missing expected log paths: %v", listed)
	}

	if status, body := requestJSONStatus(t, http.MethodGet, srv.URL+"/api/v1/accounts/"+aid+"/nginx-logs/content?path="+url.QueryEscape(vhostAccess), token, nil, nil); status != http.StatusNotFound {
		t.Fatalf("missing vhost log should 404: %d %v", status, body)
	}

	want := "GET /index.html 200\n"
	if err := os.MkdirAll(filepath.Join(root, "var/log/nginx"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "var/log/nginx", siteID+".access.log"), []byte(want), 0o640); err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/accounts/"+aid+"/nginx-logs/content?path="+url.QueryEscape(vhostAccess), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("download status %d %s", res.StatusCode, raw)
	}
	if string(raw) != want {
		t.Fatalf("download body %q", raw)
	}
	if !strings.Contains(res.Header.Get("Content-Disposition"), siteID+".access.log") {
		t.Fatalf("disposition: %s", res.Header.Get("Content-Disposition"))
	}

	if status, _ := requestJSONStatus(t, http.MethodGet, srv.URL+"/api/v1/accounts/"+aid+"/nginx-logs/content?path="+url.QueryEscape("/etc/passwd"), token, nil, nil); status != http.StatusBadRequest {
		t.Fatalf("foreign path must be refused: %d", status)
	}

	again := get(t, srv.URL+"/api/v1/accounts/"+aid+"/nginx-logs", token)
	for _, raw := range again["items"].([]any) {
		row := raw.(map[string]any)
		if row["path"] == vhostAccess && row["present"] != true {
			t.Fatalf("written vhost log must be present: %v", row)
		}
	}
}

func TestDemoAccountsSettingsSkipHostApply(t *testing.T) {
	srv, st, token := directorFixture(t)
	saved := doJSON(t, http.MethodPatch, srv.URL+"/api/v1/server/settings", token, map[string]any{
		"values": map[string]any{"demo_accounts": map[string]string{"usernames": "orbixtar"}},
	})
	if saved["operation_id"] != nil {
		t.Fatalf("demo set must not queue host apply: %v", saved)
	}
	for _, job := range st.ListJobs("", 50) {
		if job.Type == "host.config.apply" {
			t.Fatalf("unexpected host apply: %+v", job)
		}
	}
	got := get(t, srv.URL+"/api/v1/server/settings", token)
	row := got["values"].(map[string]any)["demo_accounts"].(map[string]any)
	if row["usernames"] != "orbixtar" {
		t.Fatalf("demo set: %v", got)
	}
}
