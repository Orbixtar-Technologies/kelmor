package operations

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagePanelUpdateAcceptsFixedActions(t *testing.T) {
	root := t.TempDir()
	host := &Host{Root: root}
	configPath := filepath.Join(root, "etc/panel/update.env")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("PANEL_UPDATE_CHANNEL=stable\nPANEL_UPDATE_AUTOMATIC=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, action := range []string{"check", "install"} {
		t.Run(action, func(t *testing.T) {
			raw, err := host.Dispatch(context.Background(), Request{
				Method: "ManagePanelUpdate",
				Params: mustRaw(map[string]any{"action": action}),
			})
			if err != nil {
				t.Fatal(err)
			}
			result, ok := raw.(Result)
			if !ok || !result.OK || result.ObservedState != action+"-requested" {
				t.Fatalf("result = %#v", raw)
			}
		})
	}

	automatic := false
	raw, err := host.Dispatch(context.Background(), Request{
		Method: "ManagePanelUpdate",
		Params: mustRaw(map[string]any{"action": "settings", "automatic": automatic}),
	})
	if err != nil {
		t.Fatal(err)
	}
	result, ok := raw.(Result)
	if !ok || !result.OK || result.ObservedState != "automatic-disabled" {
		t.Fatalf("result = %#v", raw)
	}
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(config); got != "PANEL_UPDATE_CHANNEL=stable\nPANEL_UPDATE_AUTOMATIC=false\n" {
		t.Fatalf("config = %q", got)
	}
}

func TestManagePanelUpdateRejectsCommandsURLsAndPaths(t *testing.T) {
	host := &Host{Root: t.TempDir()}
	automatic := true
	requests := []map[string]any{
		{"action": "shell"},
		{"action": "check", "command": "id"},
		{"action": "check", "url": "https://attacker.invalid"},
		{"action": "check", "key": "attacker-key"},
		{"action": "check", "config_path": "/tmp/update.env"},
		{"action": "check", "status_path": "/tmp/status.json"},
		{"action": "install", "install_root": "/tmp/panel"},
		{"action": "check", "automatic": automatic},
		{"action": "settings"},
	}

	for _, params := range requests {
		params := params
		t.Run(testUpdateRequestName(params), func(t *testing.T) {
			if _, err := host.Dispatch(context.Background(), Request{
				Method: "ManagePanelUpdate",
				Params: mustRaw(params),
			}); err == nil {
				t.Fatalf("accepted unsafe request: %#v", params)
			}
		})
	}
}

func testUpdateRequestName(params map[string]any) string {
	raw, _ := json.Marshal(params)
	return strings.NewReplacer("/", "_", " ", "_").Replace(string(raw))
}
