package httpserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/id"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/store"
)

func TestMailQueueShowReturnsEmptyList(t *testing.T) {
	srv, _, token := directorFixture(t)
	status, body := doStatus(t, http.MethodGet, srv.URL+"/api/v1/mail/queue", token, nil)
	if status != http.StatusOK {
		t.Fatalf("mail queue GET %d %v", status, body)
	}
	items, _ := body["items"].([]any)
	if items == nil {
		t.Fatalf("items must be an array, got %v", body)
	}
	if body["error"] != nil {
		t.Fatalf("empty queue must not be an error: %v", body)
	}
	if body["partial"] == true {
		t.Fatalf("idle sandbox queue must be a true empty list, not a read failure: %v", body)
	}
}

func TestFirewallMetadataNamesNftablesNotCSF(t *testing.T) {
	srv, _, token := directorFixture(t)
	status, body := doStatus(t, http.MethodGet, srv.URL+"/api/v1/server/firewall", token, nil)
	if status != http.StatusOK {
		t.Fatalf("firewall GET %d %v", status, body)
	}
	if body["table"] != "inet panel" {
		t.Fatalf("table %v", body)
	}
	if body["stack"] != "nftables" {
		t.Fatalf("stack %v", body)
	}
	if body["csf_installed"] != false {
		t.Fatalf("csf_installed %v", body)
	}
}

func TestRedirectsHotlinkImagesAndGitEmptyStates(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	host := &operations.Host{Root: t.TempDir()}
	api := New(st, logging.New("test"), host)
	srv := httptest.NewServer(api.Handler())
	t.Cleanup(srv.Close)
	token := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})["token"].(string)
	pkg := firstPackageID(t, srv, token)
	aid := seedAccount(t, srv, token, "shop", "shop.test", pkg)
	acc := st.GetAccount(aid)
	if acc == nil {
		t.Fatal("missing account")
	}
	dom := firstAccountDomain(t, st, aid)
	site := &store.Website{
		ID: id.New(), AccountID: aid, DomainID: dom.ID, Runtime: "php",
		DocumentRoot: acc.HomePath + "/public_html", Enabled: true, DesiredRevision: 1,
	}
	st.PutWebsite(site)

	emptyRedirects := get(t, srv.URL+"/api/v1/accounts/"+aid+"/redirects", token)
	if items, _ := emptyRedirects["items"].([]any); items == nil || len(items) != 0 {
		t.Fatalf("empty redirects %v", emptyRedirects)
	}

	created := post(t, srv.URL+"/api/v1/accounts/"+aid+"/redirects", token, map[string]any{
		"website_id": site.ID, "source": "/old", "target": "https://shop.test/new", "status": 301,
	})
	if created["id"] == "" {
		t.Fatalf("create redirect %v", created)
	}
	listed := get(t, srv.URL+"/api/v1/accounts/"+aid+"/redirects", token)
	items, _ := listed["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("listed redirects %v", listed)
	}

	hotlink := get(t, srv.URL+"/api/v1/accounts/"+aid+"/websites/"+site.ID+"/hotlink", token)
	if hotlink["enabled"] != false {
		t.Fatalf("default hotlink %v", hotlink)
	}
	saved := doJSON(t, http.MethodPut, srv.URL+"/api/v1/accounts/"+aid+"/websites/"+site.ID+"/hotlink", token, map[string]any{
		"enabled": true, "allow_direct": true, "extensions": []string{"jpg", "png"},
		"allowed_referers": []string{"shop.test"},
	})
	if saved["enabled"] != true {
		t.Fatalf("saved hotlink %v", saved)
	}

	images := get(t, srv.URL+"/api/v1/accounts/"+aid+"/images", token)
	if imgItems, _ := images["items"].([]any); imgItems == nil {
		t.Fatalf("images must be an array: %v", images)
	}

	git := get(t, srv.URL+"/api/v1/accounts/"+aid+"/git", token)
	if gitItems, _ := git["items"].([]any); gitItems == nil || len(gitItems) != 0 {
		t.Fatalf("empty git %v", git)
	}
	repo := filepath.Join(host.Root, strings.TrimPrefix(acc.HomePath, "/"), "app", ".git")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	found := get(t, srv.URL+"/api/v1/accounts/"+aid+"/git", token)
	if gitItems, _ := found["items"].([]any); len(gitItems) != 1 {
		t.Fatalf("discovered git %v", found)
	}
}

func firstAccountDomain(t *testing.T, st store.Store, accountID string) store.Domain {
	t.Helper()
	for _, domain := range st.ListDomains(accountID) {
		return domain
	}
	t.Fatal("account has no domain")
	return store.Domain{}
}
