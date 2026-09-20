package operations

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

	writeSignedUpdateHost(t, root, "1.0.0", "1.0.0")
	raw, err := host.Dispatch(context.Background(), Request{
		Method: "ManagePanelUpdate",
		Params: mustRaw(map[string]any{"action": "check"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	result, ok := raw.(Result)
	if !ok || !result.OK || result.ObservedState != "idle" {
		t.Fatalf("check result = %#v", raw)
	}

	raw, err = host.Dispatch(context.Background(), Request{
		Method: "ManagePanelUpdate",
		Params: mustRaw(map[string]any{"action": "install"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	result, ok = raw.(Result)
	if !ok || !result.OK || result.ObservedState != "install-requested" {
		t.Fatalf("install result = %#v", raw)
	}

	automatic := false
	raw, err = host.Dispatch(context.Background(), Request{
		Method: "ManagePanelUpdate",
		Params: mustRaw(map[string]any{"action": "settings", "automatic": automatic}),
	})
	if err != nil {
		t.Fatal(err)
	}
	result, ok = raw.(Result)
	if !ok || !result.OK || result.ObservedState != "automatic-disabled" {
		t.Fatalf("result = %#v", raw)
	}
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "PANEL_UPDATE_AUTOMATIC=false") ||
		strings.Contains(string(config), "PANEL_UPDATE_AUTOMATIC=true") {
		t.Fatalf("config = %q", config)
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

func TestPanelUpdateActionsSelectFixedUnits(t *testing.T) {
	checkArgs, err := panelUpdateStartArgs("check")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(checkArgs, []string{"start", "panel-update@check.service"}) {
		t.Fatalf("check systemctl args = %q", checkArgs)
	}
	installArgs, err := panelUpdateStartArgs("install")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(installArgs, []string{"start", "--no-block", "panel-update@install.service"}) {
		t.Fatalf("install systemctl args = %q", installArgs)
	}
	if _, err := panelUpdateStartArgs("panel-update@attacker.service"); err == nil {
		t.Fatal("caller-controlled unit was accepted")
	}
}

func TestManagePanelUpdateCheckPersistsLastCheckedAt(t *testing.T) {
	root := t.TempDir()
	host := &Host{Root: root}
	writeSignedUpdateHost(t, root, "1.0.0", "1.0.0")

	result, err := host.ManagePanelUpdate(context.Background(), PanelUpdateRequest{Action: "check"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.ObservedState != "idle" {
		t.Fatalf("result = %#v", result)
	}

	raw, err := os.ReadFile(filepath.Join(root, "var/lib/panel/update-status.json"))
	if err != nil {
		t.Fatal(err)
	}
	var status update.Status
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatal(err)
	}
	if status.LastCheckedAt == "" {
		t.Fatalf("check did not persist last_checked_at: %+v", status)
	}
	if _, err := time.Parse(time.RFC3339, status.LastCheckedAt); err != nil {
		t.Fatalf("last_checked_at %q: %v", status.LastCheckedAt, err)
	}
	if status.State != "idle" {
		t.Fatalf("status = %+v", status)
	}
}

func TestManagePanelUpdateCheckFailureLeavesLastCheckedAt(t *testing.T) {
	root := t.TempDir()
	host := &Host{Root: root}
	previous := "2026-09-09T16:00:00Z"
	writeSignedUpdateHost(t, root, "1.0.0", "1.0.0")
	statusPath := filepath.Join(root, "var/lib/panel/update-status.json")
	if err := update.WriteStatus(statusPath, update.Status{
		State: "idle", InstalledRelease: "1.0.0", LastCheckedAt: previous,
		Automatic: true, Channel: "stable",
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "etc/panel/update.env"), []byte(
		"PANEL_UPDATE_FEED_URL=http://updates.example.test\n"+
			"PANEL_UPDATE_CHANNEL=stable\n"+
			"PANEL_UPDATE_AUTOMATIC=true\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := host.ManagePanelUpdate(context.Background(), PanelUpdateRequest{Action: "check"}); err == nil {
		t.Fatal("expected check failure")
	}

	raw, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	var status update.Status
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatal(err)
	}
	if status.LastCheckedAt != previous {
		t.Fatalf("failed check overwrote last_checked_at: %+v", status)
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

func writeSignedUpdateHost(t *testing.T, root, installed, available string) *httptest.Server {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("panel")
	sum := sha256.Sum256(payload)
	manifest := &update.Manifest{
		Release: available, Channel: "stable", MinimumRelease: "1.0.0",
		Artifacts: []update.Artifact{{
			Path: "panel-api", Target: "bin/panel-api",
			Size: int64(len(payload)), SHA256: hex.EncodeToString(sum[:]), Mode: 0o755,
		}},
	}
	if err := update.Sign(manifest, priv); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stable/manifest.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	t.Cleanup(server.Close)

	if err := os.MkdirAll(filepath.Join(root, "usr/local/panel"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "usr/local/panel/current-release"), []byte(installed+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "etc/panel"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "etc/panel/update.pub"), []byte(hex.EncodeToString(pub)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := "PANEL_UPDATE_FEED_URL=" + server.URL + "\n" +
		"PANEL_UPDATE_CHANNEL=stable\n" +
		"PANEL_UPDATE_AUTOMATIC=true\n"
	if err := os.WriteFile(filepath.Join(root, "etc/panel/update.env"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := update.WriteStatus(filepath.Join(root, "var/lib/panel/update-status.json"), update.Status{
		State: "idle", InstalledRelease: installed, Automatic: true, Channel: "stable",
	}); err != nil {
		t.Fatal(err)
	}
	return server
}
