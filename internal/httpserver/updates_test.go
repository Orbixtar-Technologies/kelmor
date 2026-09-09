package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/auth"
	"github.com/hosting-panel/panel/internal/id"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/rbac"
	"github.com/hosting-panel/panel/internal/store"
	"github.com/hosting-panel/panel/internal/update"
)

func TestUpdateStatusRequiresServerRead(t *testing.T) {
	server := newUpdateTestServer(t)
	statusPath := filepath.Join(server.root, "var/lib/panel/update-status.json")
	if err := update.WriteStatus(statusPath, update.Status{
		State: "available", InstalledRelease: "1.0.0", AvailableRelease: "1.1.0",
		Automatic: true, Channel: "stable",
	}); err != nil {
		t.Fatal(err)
	}

	code, body := requestJSONStatus(t, http.MethodGet, server.url+"/api/v1/server/updates", server.auditor, nil, nil)
	if code != http.StatusOK || body["installed_release"] != "1.0.0" || body["available_release"] != "1.1.0" {
		t.Fatalf("authorized status: %d %v", code, body)
	}
	code, body = requestJSONStatus(t, http.MethodGet, server.url+"/api/v1/server/updates", server.customer, nil, nil)
	assertAPIErrorCode(t, code, body, http.StatusForbidden, "FORBIDDEN")
	code, body = requestJSONStatus(t, http.MethodGet, server.url+"/api/v1/server/updates", "", nil, nil)
	assertAPIErrorCode(t, code, body, http.StatusUnauthorized, "UNAUTHENTICATED")
}

func TestUpdateMutationsRequireServerSettingsWrite(t *testing.T) {
	server := newUpdateTestServer(t)
	requests := []struct {
		method string
		path   string
		body   any
	}{
		{method: http.MethodPost, path: "/api/v1/server/updates/check"},
		{method: http.MethodPost, path: "/api/v1/server/updates/install"},
		{method: http.MethodPatch, path: "/api/v1/server/updates/settings", body: map[string]any{"automatic": false}},
	}

	for _, request := range requests {
		code, body := requestJSONStatus(t, request.method, server.url+request.path, server.auditor, request.body, nil)
		assertAPIErrorCode(t, code, body, http.StatusForbidden, "FORBIDDEN")
	}
}

func TestUpdateMutationsRejectAccountScopedToken(t *testing.T) {
	server := newUpdateTestServer(t)
	plain := "hp_live_update_account_scope"
	server.store.PutToken(&store.APIToken{
		ID: id.New(), UserID: server.adminID, Name: "unsafe-update-token",
		TokenHash: auth.HashToken(plain), Scope: "account", AccountID: id.New(),
		Capabilities: []string{rbac.ServerSettingsWrite},
	})

	code, body := requestJSONStatus(
		t,
		http.MethodPost,
		server.url+"/api/v1/server/updates/check",
		plain,
		nil,
		nil,
	)
	assertAPIErrorCode(t, code, body, http.StatusForbidden, "FORBIDDEN")
	if hasSuccessfulAudit(server.store, "server.update.check") {
		t.Fatal("rejected account-scoped update was audited as successful")
	}
}

func TestUpdateMutationIsAudited(t *testing.T) {
	server := newUpdateTestServer(t)
	code, body := requestJSONStatus(
		t,
		http.MethodPost,
		server.url+"/api/v1/server/updates/check",
		server.admin,
		nil,
		map[string]string{"X-Request-ID": "update-audit-request"},
	)
	if code != http.StatusAccepted {
		t.Fatalf("check update: %d %v", code, body)
	}

	for _, event := range server.store.ListAudit(20) {
		if event.Action != "server.update.check" {
			continue
		}
		encoded, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range [][]byte{
			[]byte("url"), []byte("key"), []byte("command"), []byte("config_path"),
			[]byte("status_path"), []byte("install_root"),
		} {
			if bytes.Contains(encoded, forbidden) {
				t.Fatalf("audit contains privileged input %q: %s", forbidden, encoded)
			}
		}
		if !event.Success || event.ResourceType != "server_update" || event.RequestID != "update-audit-request" {
			t.Fatalf("audit event: %+v", event)
		}
		return
	}
	t.Fatal("successful update mutation was not audited")
}

func TestUpdateAPIRejectsCallerControlledPrivilegedInputs(t *testing.T) {
	server := newUpdateTestServer(t)
	configPath := filepath.Join(server.root, "etc/panel/update.env")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("PANEL_UPDATE_CHANNEL=stable\nPANEL_UPDATE_AUTOMATIC=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	requests := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodPost, "/api/v1/server/updates/check", map[string]any{"url": "https://attacker.invalid"}},
		{http.MethodPost, "/api/v1/server/updates/check", map[string]any{"key": "attacker-key"}},
		{http.MethodPost, "/api/v1/server/updates/check", map[string]any{"command": "id"}},
		{http.MethodPost, "/api/v1/server/updates/install", map[string]any{"install_root": "/tmp/panel"}},
		{http.MethodPatch, "/api/v1/server/updates/settings", map[string]any{"automatic": true, "config_path": "/tmp/update.env"}},
		{http.MethodPatch, "/api/v1/server/updates/settings", map[string]any{"automatic": true, "status_path": "/tmp/status.json"}},
		{http.MethodPatch, "/api/v1/server/updates/settings", map[string]any{"automatic": "yes"}},
	}
	for _, request := range requests {
		code, body := requestJSONStatus(t, request.method, server.url+request.path, server.admin, request.body, nil)
		assertAPIErrorCode(t, code, body, http.StatusBadRequest, "INVALID_JSON")
	}
}

type updateTestServer struct {
	store    store.Store
	root     string
	url      string
	adminID  string
	admin    string
	auditor  string
	customer string
}

func newUpdateTestServer(t *testing.T) updateTestServer {
	t.Helper()
	data := store.NewMemory()
	if err := store.SeedDev(data, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	putUpdateTestUser(t, data, "update-auditor", "auditor")
	putUpdateTestUser(t, data, "update-customer", "customer_owner")

	root := t.TempDir()
	api := New(data, logging.New("test"), &operations.Host{Root: root})
	server := httptest.NewServer(api.Handler())
	t.Cleanup(server.Close)

	return updateTestServer{
		store: data, root: root, url: server.URL,
		adminID:  data.UserByUsername("admin").ID,
		admin:    loginUpdateTestUser(t, server.URL, "admin", "ChangeMeOnce!2026"),
		auditor:  loginUpdateTestUser(t, server.URL, "update-auditor", "UpdateTestPass!2026"),
		customer: loginUpdateTestUser(t, server.URL, "update-customer", "UpdateTestPass!2026"),
	}
}

func putUpdateTestUser(t *testing.T, data store.Store, username, role string) {
	t.Helper()
	passwordHash, err := auth.HashPassword("UpdateTestPass!2026")
	if err != nil {
		t.Fatal(err)
	}
	data.PutUser(&store.User{
		ID: id.New(), Username: username, PasswordHash: passwordHash,
		Status: "active", Roles: []string{role},
	})
}

func loginUpdateTestUser(t *testing.T, url, username, password string) string {
	t.Helper()
	return post(t, url+"/api/v1/auth/login", "", map[string]string{
		"username": username,
		"password": password,
	})["token"].(string)
}
