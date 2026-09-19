package httpserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/auth"
	"github.com/hosting-panel/panel/internal/id"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/store"
)

// directorAPIWithStateDir boots like panel-api.service: PANEL_STATE_DIR is set
// and the agent socket is present, so settings must not be rewritten under
// Agent.Root.
func directorAPIWithStateDir(t *testing.T, stateDir string) (*httptest.Server, store.Store, string) {
	t.Helper()
	t.Setenv("PANEL_STATE_DIR", stateDir)
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Sock: "/run/panel/agent.sock"})
	srv := httptest.NewServer(api.Handler())
	t.Cleanup(srv.Close)
	login := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})
	return srv, st, login["token"].(string)
}

func TestDirectorSettingsRoundTripAndRejectSecrets(t *testing.T) {
	srv, _, token := directorFixture(t)

	empty := get(t, srv.URL+"/api/v1/server/settings", token)
	if values, _ := empty["values"].(map[string]any); len(values) != 0 {
		t.Fatalf("expected empty settings, got %v", empty)
	}

	saved := doJSON(t, http.MethodPatch, srv.URL+"/api/v1/server/settings", token, map[string]any{
		"values": map[string]any{
			"tweak_settings": map[string]string{"max_emails_hour": "250", "default_php": "8.3"},
		},
	})
	if saved["operation_id"] == nil || saved["operation_id"] == "" {
		t.Fatalf("host apply job missing: %v", saved)
	}
	got := saved["values"].(map[string]any)["tweak_settings"].(map[string]any)
	if got["max_emails_hour"] != "250" || got["default_php"] != "8.3" {
		t.Fatalf("saved settings: %v", saved)
	}

	again := get(t, srv.URL+"/api/v1/server/settings", token)
	persisted := again["values"].(map[string]any)["tweak_settings"].(map[string]any)
	if persisted["max_emails_hour"] != "250" {
		t.Fatalf("settings did not persist: %v", again)
	}

	status, body := doStatus(t, http.MethodPatch, srv.URL+"/api/v1/server/settings", token, map[string]any{
		"values": map[string]any{
			"mysql_root": map[string]string{"password": "please-no"},
		},
	})
	if status != http.StatusBadRequest {
		t.Fatalf("secrets must be rejected: %d %v", status, body)
	}
}

func TestDirectorSettingsPersistWhenStateDirNotWritable(t *testing.T) {
	state := t.TempDir()
	control := filepath.Join(state, "control")
	if err := os.MkdirAll(control, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(state, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(state, 0o755) })

	srv, st, token := directorAPIWithStateDir(t, state)
	saved := doJSON(t, http.MethodPatch, srv.URL+"/api/v1/server/settings", token, map[string]any{
		"values": map[string]any{
			"tweak_settings": map[string]string{"max_emails_hour": "250", "default_php": "8.3"},
		},
	})
	if saved["operation_id"] == nil || saved["operation_id"] == "" {
		t.Fatalf("host apply job missing: %v", saved)
	}
	job := st.GetJob(saved["operation_id"].(string))
	if job == nil || job.Type != "host.config.apply" {
		t.Fatalf("queued job: %+v", job)
	}
	if _, err := os.Stat(filepath.Join(state, "director-settings.json")); err == nil {
		t.Fatal("must not write director-settings.json into the unwritable state dir")
	}
	raw, err := os.ReadFile(filepath.Join(control, "director-settings.json"))
	if err != nil {
		t.Fatalf("expected persist under control/: %v", err)
	}
	if !strings.Contains(string(raw), `"max_emails_hour": "250"`) {
		t.Fatalf("control file: %s", raw)
	}
}

func TestDirectorSettingsLoadLegacyAndWriteControl(t *testing.T) {
	state := t.TempDir()
	legacy := []byte(`{"values":{"tweak_settings":{"max_emails_hour":"100","default_php":8.3,"allow_parked":true}}}`)
	if err := os.WriteFile(filepath.Join(state, "director-settings.json"), legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	srv, st, token := directorAPIWithStateDir(t, state)
	got := get(t, srv.URL+"/api/v1/server/settings", token)
	values, _ := got["values"].(map[string]any)
	row, _ := values["tweak_settings"].(map[string]any)
	if row["max_emails_hour"] != "100" || row["default_php"] != "8.3" || row["allow_parked"] != "on" {
		t.Fatalf("legacy load with mixed types: %v", got)
	}

	saved := doJSON(t, http.MethodPatch, srv.URL+"/api/v1/server/settings", token, map[string]any{
		"values": map[string]any{
			"tweak_settings": map[string]string{"max_emails_hour": "250", "default_php": "8.3", "allow_parked": "on"},
		},
	})
	if saved["operation_id"] == nil || saved["operation_id"] == "" {
		t.Fatalf("host apply job missing: %v", saved)
	}
	if job := st.GetJob(saved["operation_id"].(string)); job == nil || job.Type != "host.config.apply" {
		t.Fatalf("queued job: %+v", job)
	}
	if _, err := os.Stat(filepath.Join(state, "control", "director-settings.json")); err != nil {
		t.Fatalf("expected write under control/: %v", err)
	}
}

func TestDirectorSettingsPersistReportsWriteError(t *testing.T) {
	state := t.TempDir()
	control := filepath.Join(state, "control")
	if err := os.MkdirAll(control, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(control, 0o555); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(state, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(control, 0o755)
		_ = os.Chmod(state, 0o755)
	})

	srv, st, token := directorAPIWithStateDir(t, state)
	status, body := doStatus(t, http.MethodPatch, srv.URL+"/api/v1/server/settings", token, map[string]any{
		"values": map[string]any{
			"tweak_settings": map[string]string{"max_emails_hour": "250"},
		},
	})
	if status != http.StatusInternalServerError {
		t.Fatalf("expected persist failure: %d %v", status, body)
	}
	msg, _ := body["error"].(map[string]any)["message"].(string)
	if !strings.Contains(msg, "Could not persist Director settings") {
		t.Fatalf("message: %v", body)
	}
	if !strings.Contains(strings.ToLower(msg), "permission denied") && !strings.Contains(strings.ToLower(msg), "denied") {
		t.Fatalf("persist error must include the OS cause: %v", body)
	}
	for _, job := range st.ListJobs("", 50) {
		if job.Type == "host.config.apply" {
			t.Fatalf("must not queue apply when persist fails: %+v", job)
		}
	}
}

func TestDirectorSettingsRequireCapabilities(t *testing.T) {
	srv, st, _ := directorFixture(t)
	hash, err := auth.HashPassword("AuditPass!2026")
	if err != nil {
		t.Fatal(err)
	}
	st.PutUser(&store.User{
		ID: id.New(), Username: "settings-auditor", Email: "audit@localhost", PasswordHash: hash,
		DisplayName: "Auditor", Status: "active", Roles: []string{"auditor"},
	})
	auditor := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "settings-auditor", "password": "AuditPass!2026"})["token"].(string)

	if status, _ := doStatus(t, http.MethodGet, srv.URL+"/api/v1/server/settings", auditor, nil); status != http.StatusOK {
		t.Fatalf("auditor can read settings: %d", status)
	}
	if status, _ := doStatus(t, http.MethodPatch, srv.URL+"/api/v1/server/settings", auditor, map[string]any{
		"values": map[string]any{"theme": map[string]string{"density": "compact"}},
	}); status != http.StatusForbidden {
		t.Fatalf("auditor must not write settings")
	}
}

func TestModifyAccountShellClass(t *testing.T) {
	srv, st, token := directorFixture(t)
	pkg := firstPackageID(t, srv, token)
	aid := seedAccount(t, srv, token, "shelluser", "shell.example.test", pkg)

	updated := doJSON(t, http.MethodPatch, srv.URL+"/api/v1/accounts/"+aid, token, map[string]any{
		"shell_class": "jailed",
	})
	account := updated["account"].(map[string]any)
	if account["shell_class"] != "jailed" {
		t.Fatalf("modify response: %v", updated)
	}
	stored := st.GetAccount(aid)
	if stored == nil || stored.ShellClass != "jailed" {
		t.Fatalf("store shell_class: %+v", stored)
	}

	if status, body := doStatus(t, http.MethodPatch, srv.URL+"/api/v1/accounts/"+aid, token, map[string]any{
		"shell_class": "root",
	}); status != http.StatusBadRequest {
		t.Fatalf("invalid shell must be refused: %d %v", status, body)
	}
}
