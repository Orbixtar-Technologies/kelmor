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
