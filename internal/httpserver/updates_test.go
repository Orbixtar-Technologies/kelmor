package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hosting-panel/panel/agent/operations"
	openapi "github.com/hosting-panel/panel/api"
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

	cookieRequest, err := http.NewRequest(http.MethodGet, server.url+"/api/v1/server/updates", nil)
	if err != nil {
		t.Fatal(err)
	}
	cookieRequest.AddCookie(loginUpdateTestCookie(t, server.url))
	cookieResponse, err := http.DefaultClient.Do(cookieRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer cookieResponse.Body.Close()
	if cookieResponse.StatusCode != http.StatusOK {
		t.Fatalf("cookie-authenticated status GET = %d", cookieResponse.StatusCode)
	}
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

	var intent *store.AuditEvent
	var completion *store.AuditEvent
	for _, event := range server.store.ListAudit(20) {
		switch event.Action {
		case "server.update.check.intent":
			candidate := event
			intent = &candidate
		case "server.update.check":
			candidate := event
			completion = &candidate
		default:
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
	}
	if intent == nil || completion == nil {
		t.Fatalf("intent and completion audits are required: %+v", server.store.ListAudit(20))
	}
	if intent.OccurredAt.After(completion.OccurredAt) {
		t.Fatalf("intent audit occurred after completion: intent=%s completion=%s", intent.OccurredAt, completion.OccurredAt)
	}
}

func TestUpdateMutationIntentIsAuditedBeforeAgentFailure(t *testing.T) {
	server := newUpdateTestServer(t)
	code, body := requestJSONStatus(
		t,
		http.MethodPatch,
		server.url+"/api/v1/server/updates/settings",
		server.admin,
		map[string]any{"automatic": false},
		map[string]string{"X-Request-ID": "failed-update-request"},
	)
	assertAPIErrorCode(t, code, body, http.StatusInternalServerError, "AGENT_ERROR")

	foundIntent := false
	for _, event := range server.store.ListAudit(20) {
		if event.Action == "server.update.settings.intent" &&
			event.RequestID == "failed-update-request" && event.Success {
			foundIntent = true
		}
		if event.Action == "server.update.settings" &&
			event.RequestID == "failed-update-request" && event.Success {
			t.Fatal("failed privileged mutation has a completion audit")
		}
	}
	if !foundIntent {
		t.Fatal("privileged mutation failure has no prior intent audit")
	}
}

func TestUpdateMutationsRequireBearerAndSameOrigin(t *testing.T) {
	server := newUpdateTestServer(t)
	sessionCookie := loginUpdateTestCookie(t, server.url)
	if sessionCookie.Secure {
		t.Fatal("HTTP development login issued a Secure cookie")
	}

	cookieOnly, err := http.NewRequest(http.MethodPost, server.url+"/api/v1/server/updates/check", nil)
	if err != nil {
		t.Fatal(err)
	}
	cookieOnly.AddCookie(sessionCookie)
	cookieOnly.Header.Set("Origin", "https://attacker.invalid")
	response, err := http.DefaultClient.Do(cookieOnly)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(response.Body).Decode(&body)
	assertAPIErrorCode(t, response.StatusCode, body, http.StatusUnauthorized, "BEARER_REQUIRED")

	code, body := requestJSONStatus(
		t,
		http.MethodPost,
		server.url+"/api/v1/server/updates/check",
		server.admin,
		nil,
		map[string]string{"Origin": server.url},
	)
	if code != http.StatusAccepted {
		t.Fatalf("same-origin bearer update: %d %v", code, body)
	}
	code, body = requestJSONStatus(
		t,
		http.MethodPost,
		server.url+"/api/v1/server/updates/check",
		server.admin,
		nil,
		map[string]string{"Origin": "https://attacker.invalid"},
	)
	assertAPIErrorCode(t, code, body, http.StatusForbidden, "ORIGIN_FORBIDDEN")
}

func TestUpdateSettingsAreReflectedInStatus(t *testing.T) {
	server := newUpdateTestServer(t)
	configPath := filepath.Join(server.root, "etc/panel/update.env")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("PANEL_UPDATE_CHANNEL=stable\nPANEL_UPDATE_AUTOMATIC=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	statusPath := filepath.Join(server.root, "var/lib/panel/update-status.json")
	if err := update.WriteStatus(statusPath, update.Status{
		State: "idle", InstalledRelease: "1.0.0", Automatic: true, Channel: "stable",
	}); err != nil {
		t.Fatal(err)
	}

	code, body := requestJSONStatus(
		t,
		http.MethodPatch,
		server.url+"/api/v1/server/updates/settings",
		server.admin,
		map[string]any{"automatic": false},
		nil,
	)
	if code != http.StatusOK {
		t.Fatalf("settings update: %d %v", code, body)
	}
	code, body = requestJSONStatus(
		t,
		http.MethodGet,
		server.url+"/api/v1/server/updates",
		server.auditor,
		nil,
		nil,
	)
	if code != http.StatusOK || body["automatic"] != false {
		t.Fatalf("status after settings update: %d %v", code, body)
	}
}

func TestLoginCookieIsSecureForHTTPS(t *testing.T) {
	data := store.NewMemory()
	if err := store.SeedDev(data, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(data, logging.New("test"), &operations.Host{Root: t.TempDir()})
	body, err := json.Marshal(map[string]string{
		"username": "admin",
		"password": "ChangeMeOnce!2026",
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "https://panel.test/api/v1/auth/login", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	api.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("login status %d: %s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) == 0 || !cookies[0].Secure {
		t.Fatalf("HTTPS login cookie is not Secure: %+v", cookies)
	}
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

func TestUpdateOpenAPIDocumentsSecurityAndAsyncResponses(t *testing.T) {
	document := string(openapi.YAML)
	if !strings.Contains(document, "securitySchemes:") ||
		!strings.Contains(document, "bearerAuth:") ||
		!strings.Contains(document, "cookieAuth:") {
		t.Fatal("OpenAPI is missing bearer and cookie security schemes")
	}

	status := openAPIPathBlock(document, "/server/updates")
	if !strings.Contains(status, "bearerAuth: []") ||
		!strings.Contains(status, "cookieAuth: []") ||
		!strings.Contains(status, `"401": { $ref: "#/components/responses/Error" }`) ||
		!strings.Contains(status, `$ref: "#/components/schemas/UpdateStatus"`) {
		t.Fatalf("update status security/schema is incomplete:\n%s", status)
	}

	for _, path := range []string{"/server/updates/check", "/server/updates/install"} {
		block := openAPIPathBlock(document, path)
		if !strings.Contains(block, "security:\n        - bearerAuth: []") ||
			strings.Contains(block, "cookieAuth: []") ||
			!strings.Contains(block, `"202":`) ||
			!strings.Contains(block, `$ref: "#/components/schemas/PanelUpdateResult"`) ||
			!strings.Contains(block, `"401": { $ref: "#/components/responses/Error" }`) {
			t.Fatalf("%s security/202 schema is incomplete:\n%s", path, block)
		}
	}

	settings := openAPIPathBlock(document, "/server/updates/settings")
	if !strings.Contains(settings, "security:\n        - bearerAuth: []") ||
		strings.Contains(settings, "cookieAuth: []") ||
		!strings.Contains(settings, `"200":`) ||
		!strings.Contains(settings, `$ref: "#/components/schemas/PanelUpdateResult"`) ||
		!strings.Contains(settings, `"401": { $ref: "#/components/responses/Error" }`) {
		t.Fatalf("update settings security/schema is incomplete:\n%s", settings)
	}
}

func openAPIPathBlock(document, path string) string {
	start := strings.Index(document, "  "+path+":\n")
	if start < 0 {
		return ""
	}
	remaining := document[start+1:]
	end := strings.Index(remaining, "\n  /")
	if end < 0 {
		return remaining
	}
	return remaining[:end]
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

func loginUpdateTestCookie(t *testing.T, url string) *http.Cookie {
	t.Helper()
	body, err := json.Marshal(map[string]string{
		"username": "admin",
		"password": "ChangeMeOnce!2026",
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(
		http.MethodPost,
		url+"/api/v1/auth/login",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	for _, cookie := range response.Cookies() {
		if cookie.Name == "panel_session" {
			return cookie
		}
	}
	t.Fatal("login did not issue session cookie")
	return nil
}
