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

func seedPHPFPM(t *testing.T, host *operations.Host, versions ...string) {
	t.Helper()
	dir := filepath.Join(host.Root, "usr/sbin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, version := range versions {
		if err := os.WriteFile(filepath.Join(dir, "php-fpm"+version), []byte(""), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCreateWebsiteRejectsUninstalledPHP(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	host := &operations.Host{Root: t.TempDir()}
	seedPHPFPM(t, host, "8.3")
	api := New(st, logging.New("test"), host)
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})["token"].(string)
	pkg := get(t, srv.URL+"/api/v1/packages", admin)["items"].([]any)[0].(map[string]any)["id"].(string)
	acc := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "phpone", "primary_domain": "phpone.test", "package_id": pkg,
		"owner_email": "o@phpone.test", "owner_password": "TenantPass!2026",
	})
	aid := acc["resource_id"].(string)
	d := &store.Domain{ID: id.New(), AccountID: aid, FQDN: "app.phpone.test", ASCII: "app.phpone.test", Type: "addon"}
	st.PutDomain(d)

	code, body := postStatus(t, srv.URL+"/api/v1/accounts/"+aid+"/websites", admin, map[string]any{
		"domain_id": d.ID, "runtime": "php", "runtime_version": "8.4",
		"document_root": "/home/phpone/app.phpone.test",
	})
	if code != 400 {
		t.Fatalf("uninstalled 8.4 should be rejected, got %d %v", code, body)
	}
	msg, _ := body["error"].(map[string]any)["message"].(string)
	if !strings.Contains(msg, "php-fpm 8.4 is not installed") {
		t.Fatalf("error %v", body)
	}

	ok := post(t, srv.URL+"/api/v1/accounts/"+aid+"/websites", admin, map[string]any{
		"domain_id": d.ID, "runtime": "php", "runtime_version": "8.3",
		"document_root": "/home/phpone/app.phpone.test",
	})
	if ok["operation_id"] == nil {
		t.Fatalf("installed 8.3 should queue: %v", ok)
	}
}

func TestInstalledPHPVersionsFromFiltersHostCatalog(t *testing.T) {
	got := installedPHPVersionsFrom([]map[string]any{
		{"version": "8.3", "status": "installed"},
		{"version": "8.4", "status": "available"},
		{"version": "8.1", "status": "installed"},
	})
	if len(got) != 1 || got[0] != "8.3" {
		t.Fatalf("installed filter %v", got)
	}
	if defaultInstalledPHP(got) != "8.3" {
		t.Fatalf("default %q", defaultInstalledPHP(got))
	}
	if phpVersionInstalled(got, "8.4") {
		t.Fatal("8.4 is not installed")
	}
}

func TestListPHPRuntimesAllowsWebsitesRead(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	putUpdateTestUser(t, st, "tenant-php", "customer_owner")
	host := &operations.Host{Root: t.TempDir()}
	seedPHPFPM(t, host, "8.3")
	api := New(st, logging.New("test"), host)
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	token := loginUpdateTestUser(t, srv.URL, "tenant-php", "UpdateTestPass!2026")

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/server/runtimes", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("websites.read should list PHP inventory, got %d", res.StatusCode)
	}
}
