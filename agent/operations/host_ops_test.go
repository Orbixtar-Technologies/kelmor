package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetMariaDBRootPasswordRejectsDashPrefix(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if _, err := h.setMariaDBRootPassword("", "-DbRoot!2026"); err == nil {
		t.Fatal("expected dash-prefix password to be rejected before live()")
	}
	if _, err := h.setMariaDBRootPassword("-current", "DbRoot!2026"); err == nil {
		t.Fatal("expected dash-prefix current password to be rejected")
	}
	res, err := h.setMariaDBRootPassword("", "DbRoot!2026")
	if err != nil || res.ObservedState != "staged" {
		t.Fatalf("valid staged password: %v %#v", err, res)
	}
}

func TestRedactProcessCommandHidesPasswordArgv(t *testing.T) {
	cmd := redactProcessCommand("mysqladmin", "mysqladmin -uroot -pSecret password next")
	if strings.Contains(cmd, "Secret") || strings.Contains(cmd, "next") {
		t.Fatalf("password leaked: %s", cmd)
	}
	if redactProcessCommand("nginx", "nginx -t") != "nginx -t" {
		t.Fatal("unrelated commands must stay visible")
	}
	psql := redactProcessCommand("psql", "psql -c CREATE USER shop PASSWORD 'TenantSecret'")
	if strings.Contains(psql, "TenantSecret") {
		t.Fatalf("psql password leaked: %s", psql)
	}
	runuser := redactProcessCommand("runuser", "runuser -u postgres -- psql -c ALTER USER shop PASSWORD 'x'")
	if strings.Contains(runuser, "PASSWORD") {
		t.Fatalf("runuser password leaked: %s", runuser)
	}
}

func TestEnsurePHPRuntimeRejectsUnknownVersion(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if _, err := h.ensurePHPRuntime("7.4"); err == nil {
		t.Fatal("expected unsupported version")
	}
}

func TestListHostAppsNeverLeavesPluginActionsEmpty(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(h.Root, "usr/bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.Root, "usr/bin/rspamd"), []byte("ok"), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := h.Dispatch(context.Background(), Request{Method: "ListHostApps"})
	if err != nil {
		t.Fatal(err)
	}
	apps := decodeHostAppMaps(t, raw)
	rspamd := findHostAppMap(t, apps, "rspamd")
	if rspamd["status"] != "installed" {
		t.Fatalf("planted binary should be installed: %v", rspamd)
	}
	actions, _ := rspamd["actions"].([]any)
	if len(actions) == 0 {
		t.Fatalf("installed rspamd must expose actions: %v", rspamd)
	}
	ids := map[string]bool{}
	for _, rawAction := range actions {
		action, _ := rawAction.(map[string]any)
		ids[fmt.Sprint(action["id"])] = true
		if action["label"] == "" {
			t.Fatalf("action missing label: %v", action)
		}
	}
	for _, want := range []string{"restart", "review", "status"} {
		if !ids[want] {
			t.Fatalf("missing %s in %v", want, actions)
		}
	}
	if !ids["enable"] && !ids["disable"] {
		t.Fatalf("missing enable/disable in %v", actions)
	}

	missing := &Host{Root: t.TempDir()}
	available, err := missing.Dispatch(context.Background(), Request{Method: "ListHostApps"})
	if err != nil {
		t.Fatal(err)
	}
	uninstalled := findHostAppMap(t, decodeHostAppMaps(t, available), "rspamd")
	if uninstalled["status"] != "available" {
		t.Fatalf("missing binary should be available: %v", uninstalled)
	}
	availActions, _ := uninstalled["actions"].([]any)
	if len(availActions) == 0 {
		t.Fatalf("available rspamd must still expose actions: %v", uninstalled)
	}
}

func TestControlHostAppStagedActions(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	for _, action := range []string{"restart", "enable", "disable", "install"} {
		raw, err := h.Dispatch(context.Background(), Request{
			Method: "ControlHostApp",
			Params: encodeJSON(t, map[string]any{"id": "rspamd", "action": action}),
		})
		if err != nil {
			t.Fatalf("%s: %v", action, err)
		}
		res, _ := raw.(Result)
		if !res.OK {
			t.Fatalf("%s result: %#v", action, raw)
		}
	}
	if _, err := h.Dispatch(context.Background(), Request{
		Method: "ControlHostApp",
		Params: encodeJSON(t, map[string]any{"id": "rspamd", "action": "explode"}),
	}); err == nil {
		t.Fatal("expected unknown action")
	}
	if _, err := h.Dispatch(context.Background(), Request{
		Method: "ControlHostApp",
		Params: encodeJSON(t, map[string]any{"id": "not-a-plugin", "action": "restart"}),
	}); err == nil {
		t.Fatal("expected unknown app")
	}
}

func encodeJSON(t *testing.T, value any) []byte {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func decodeHostAppMaps(t *testing.T, raw any) []map[string]any {
	t.Helper()
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var apps []map[string]any
	if err := json.Unmarshal(b, &apps); err != nil {
		t.Fatalf("apps json: %s %v", b, err)
	}
	return apps
}

func findHostAppMap(t *testing.T, apps []map[string]any, id string) map[string]any {
	t.Helper()
	for _, app := range apps {
		if app["id"] == id {
			return app
		}
	}
	t.Fatalf("missing %s in %v", id, apps)
	return nil
}
