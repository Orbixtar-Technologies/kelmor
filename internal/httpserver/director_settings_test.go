package httpserver

import (
	"net/http"
	"testing"

	"github.com/hosting-panel/panel/internal/auth"
	"github.com/hosting-panel/panel/internal/id"
	"github.com/hosting-panel/panel/internal/store"
)

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
