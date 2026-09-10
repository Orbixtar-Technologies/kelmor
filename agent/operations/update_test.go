package operations

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/hosting-panel/panel/internal/update"
)

func TestManagePanelUpdateAcceptsFixedActions(t *testing.T) {
	root := t.TempDir()
	host := &Host{Root: root}
	if err := os.MkdirAll(filepath.Join(root, "usr/local/panel"), 0o755); err != nil {
		t.Fatal(err)
	}
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

func TestPanelUpdateActionsSelectFixedAsynchronousUnits(t *testing.T) {
	tests := []struct {
		action string
		unit   string
	}{
		{action: "check", unit: "panel-update@check.service"},
		{action: "install", unit: "panel-update@install.service"},
	}
	for _, test := range tests {
		t.Run(test.action, func(t *testing.T) {
			args, err := panelUpdateStartArgs(test.action)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"start", "--no-block", test.unit}
			if !slices.Equal(args, want) {
				t.Fatalf("systemctl args = %q, want %q", args, want)
			}
		})
	}
	if _, err := panelUpdateStartArgs("panel-update@attacker.service"); err == nil {
		t.Fatal("caller-controlled unit was accepted")
	}
}

func TestManagePanelUpdateSettingsPreservesMetadataAndUpdatesStatus(t *testing.T) {
	root := t.TempDir()
	host := &Host{Root: root}
	if err := os.MkdirAll(filepath.Join(root, "usr/local/panel"), 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "etc/panel/update.env")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("PANEL_UPDATE_CHANNEL=stable\nPANEL_UPDATE_AUTOMATIC=true\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeStat := before.Sys().(*syscall.Stat_t)

	statusPath := filepath.Join(root, "var/lib/panel/update-status.json")
	wantStatus := update.Status{
		State: "available", InstalledRelease: "1.0.0", AvailableRelease: "1.1.0",
		LastCheckedAt: "2026-09-09T16:00:00Z", Automatic: true, Channel: "stable",
	}
	if err := update.WriteStatus(statusPath, wantStatus); err != nil {
		t.Fatal(err)
	}

	automatic := false
	if _, err := host.ManagePanelUpdate(context.Background(), PanelUpdateRequest{
		Action: "settings", Automatic: &automatic,
	}); err != nil {
		t.Fatal(err)
	}

	after, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	afterStat := after.Sys().(*syscall.Stat_t)
	if after.Mode().Perm() != before.Mode().Perm() ||
		afterStat.Uid != beforeStat.Uid || afterStat.Gid != beforeStat.Gid {
		t.Fatalf(
			"config metadata changed: mode %o uid %d gid %d -> mode %o uid %d gid %d",
			before.Mode().Perm(), beforeStat.Uid, beforeStat.Gid,
			after.Mode().Perm(), afterStat.Uid, afterStat.Gid,
		)
	}
	rawStatus, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	var gotStatus update.Status
	if err := json.Unmarshal(rawStatus, &gotStatus); err != nil {
		t.Fatal(err)
	}
	wantStatus.Automatic = false
	if gotStatus != wantStatus {
		t.Fatalf("status = %+v, want %+v", gotStatus, wantStatus)
	}
}

func TestManagePanelUpdateSettingsSerializesConfigAndStatus(t *testing.T) {
	root := t.TempDir()
	host := &Host{Root: root}
	installRoot := filepath.Join(root, "usr/local/panel")
	if err := os.MkdirAll(installRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "etc/panel/update.env")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("PANEL_UPDATE_CHANNEL=stable\nPANEL_UPDATE_AUTOMATIC=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	statusPath := filepath.Join(root, "var/lib/panel/update-status.json")
	if err := update.WriteStatus(statusPath, update.Status{
		State: "idle", InstalledRelease: "1.0.0", Automatic: true, Channel: "stable",
	}); err != nil {
		t.Fatal(err)
	}

	const writers = 32
	start := make(chan struct{})
	errors := make(chan error, writers)
	var group sync.WaitGroup
	for index := 0; index < writers; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			automatic := false
			_, err := host.ManagePanelUpdate(context.Background(), PanelUpdateRequest{
				Action: "settings", Automatic: &automatic,
			})
			errors <- err
		}()
	}
	close(start)
	group.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}

	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(config), "PANEL_UPDATE_AUTOMATIC=false") != 1 {
		t.Fatalf("concurrent config = %q", config)
	}
	status, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	var got update.Status
	if err := json.Unmarshal(status, &got); err != nil {
		t.Fatal(err)
	}
	if got.Automatic {
		t.Fatalf("concurrent status did not reflect config: %+v", got)
	}
	lockPath := filepath.Join(installRoot, ".update.lock")
	if info, err := os.Stat(lockPath); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("canonical advisory lock missing: %v", err)
	}
}

func TestManagePanelUpdateSettingsWaitsForUpdaterCanonicalLock(t *testing.T) {
	root := t.TempDir()
	host := &Host{Root: root}
	installRoot := filepath.Join(root, "usr/local/panel")
	if err := os.MkdirAll(installRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "etc/panel/update.env")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o750); err != nil {
		t.Fatal(err)
	}
	config := "PANEL_UPDATE_CHANNEL=stable\n" +
		"PANEL_UPDATE_INSTALL_ROOT=/usr/local/panel\n" +
		"PANEL_UPDATE_AUTOMATIC=true\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	statusPath := filepath.Join(root, "var/lib/panel/update-status.json")
	if err := update.WriteStatus(statusPath, update.Status{
		State: "idle", InstalledRelease: "1.0.0", Automatic: true, Channel: "stable",
	}); err != nil {
		t.Fatal(err)
	}

	updaterEntered := make(chan struct{})
	releaseUpdater := make(chan struct{})
	updaterResult := make(chan error, 1)
	go func() {
		_, err := update.ExecuteWithLockedConfig(
			context.Background(),
			installRoot,
			"check",
			func() (update.Config, error) {
				raw, readErr := os.ReadFile(configPath)
				if readErr != nil {
					return update.Config{}, readErr
				}
				close(updaterEntered)
				<-releaseUpdater
				return update.Config{
					FeedURL:          "http://invalid.test",
					Channel:          "stable",
					InstalledRelease: "1.0.0",
					InstallRoot:      installRoot,
					StatusPath:       statusPath,
					Automatic:        strings.Contains(string(raw), "PANEL_UPDATE_AUTOMATIC=true"),
				}, nil
			},
			lockedUpdateTestRunner{},
		)
		updaterResult <- err
	}()
	<-updaterEntered

	settingsResult := make(chan error, 1)
	go func() {
		automatic := false
		_, err := host.ManagePanelUpdate(context.Background(), PanelUpdateRequest{
			Action: "settings", Automatic: &automatic,
		})
		settingsResult <- err
	}()
	select {
	case err := <-settingsResult:
		close(releaseUpdater)
		<-updaterResult
		t.Fatalf("settings did not wait for updater lock: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseUpdater)
	if err := <-updaterResult; err == nil {
		t.Fatal("invalid test feed unexpectedly passed")
	}
	if err := <-settingsResult; err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	var status update.Status
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatal(err)
	}
	if status.State != "error" || status.Error == "" || status.Automatic {
		t.Fatalf("settings overwrote concurrent updater status: %+v", status)
	}
}

func TestManagePanelUpdateSettingsLockWinsBeforeUpdaterLoadsConfig(t *testing.T) {
	root := t.TempDir()
	host := &Host{Root: root}
	installRoot := filepath.Join(root, "usr/local/panel")
	if err := os.MkdirAll(installRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "etc/panel/update.env")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o750); err != nil {
		t.Fatal(err)
	}
	config := "PANEL_UPDATE_CHANNEL=stable\n" +
		"PANEL_UPDATE_INSTALL_ROOT=/usr/local/panel\n" +
		"PANEL_UPDATE_AUTOMATIC=true\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	statusPath := filepath.Join(root, "var/lib/panel/update-status.json")
	if err := update.WriteStatus(statusPath, update.Status{
		State: "idle", InstalledRelease: "1.0.0", Automatic: true, Channel: "stable",
	}); err != nil {
		t.Fatal(err)
	}

	settingsWritten := make(chan struct{})
	releaseSettings := make(chan struct{})
	settingsResult := make(chan error, 1)
	go func() {
		settingsResult <- update.WithInstallOperationLock(installRoot, func() error {
			if err := host.writeAutomaticUpdateSettingLocked(false, "/usr/local/panel"); err != nil {
				return err
			}
			close(settingsWritten)
			<-releaseSettings
			return nil
		})
	}()
	<-settingsWritten

	loaderCalled := make(chan struct{})
	updaterResult := make(chan error, 1)
	go func() {
		_, err := update.ExecuteWithLockedConfig(
			context.Background(),
			installRoot,
			"run",
			func() (update.Config, error) {
				close(loaderCalled)
				raw, readErr := os.ReadFile(configPath)
				if readErr != nil {
					return update.Config{}, readErr
				}
				return update.Config{
					Channel:          "stable",
					InstalledRelease: "1.0.0",
					InstallRoot:      installRoot,
					StatusPath:       statusPath,
					Automatic:        strings.Contains(string(raw), "PANEL_UPDATE_AUTOMATIC=true"),
				}, nil
			},
			lockedUpdateTestRunner{},
		)
		updaterResult <- err
	}()
	select {
	case <-loaderCalled:
		close(releaseSettings)
		t.Fatal("updater loaded config while settings held the canonical lock")
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseSettings)
	if err := <-settingsResult; err != nil {
		t.Fatal(err)
	}
	if err := <-updaterResult; err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	var status update.Status
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatal(err)
	}
	if status.State != "disabled" || status.Automatic {
		t.Fatalf("updater used stale automatic setting: %+v", status)
	}
}

type lockedUpdateTestRunner struct{}

func (lockedUpdateTestRunner) Run(context.Context, string, ...string) error {
	return nil
}

func testUpdateRequestName(params map[string]any) string {
	raw, _ := json.Marshal(params)
	return strings.NewReplacer("/", "_", " ", "_").Replace(string(raw))
}
