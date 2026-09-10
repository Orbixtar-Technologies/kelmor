package update

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestWithInstallOperationLockWaitsForCanonicalLock(t *testing.T) {
	installRoot := t.TempDir()
	held, err := acquireInstallLock(installRoot)
	if err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		close(started)
		result <- WithInstallOperationLock(installRoot, func() error {
			return nil
		})
	}()
	<-started
	select {
	case err := <-result:
		_ = held.Close()
		t.Fatalf("shared operation lock did not wait: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

func TestApplyAndRollback(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	bundle := t.TempDir()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "panel-api"), []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	payload := []byte("new-bin")
	if err := os.WriteFile(filepath.Join(bundle, "panel-api"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	m := &Manifest{Release: "1.2.0", Channel: "stable", Files: map[string]string{"panel-api": hex.EncodeToString(sum[:])}}
	body, err := canonical(m)
	if err != nil {
		t.Fatal(err)
	}
	m.Signature = hex.EncodeToString(ed25519.Sign(priv, body))
	raw, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(bundle, "manifest.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Apply(bundle, root, pub); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "bin", "panel-api"))
	if string(got) != "new-bin" {
		t.Fatalf("apply wrote %q", got)
	}
	if err := Rollback(root); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(filepath.Join(root, "bin", "panel-api"))
	if string(got) != "old" {
		t.Fatalf("rollback wrote %q", got)
	}
}

func TestApplyCreatesMissingInstallRoot(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	bundle := t.TempDir()
	payload := []byte("new-bin")
	writeTestFile(t, filepath.Join(bundle, "panel-api"), payload, 0o644)
	sum := sha256.Sum256(payload)
	manifest := &Manifest{
		Release: "1.2.0", Channel: "stable",
		Files: map[string]string{"panel-api": hex.EncodeToString(sum[:])},
	}
	if err := Sign(manifest, priv); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(bundle, "manifest.json"), raw, 0o644)
	root := filepath.Join(t.TempDir(), "missing", "panel")

	if err := Apply(bundle, root, pub); err != nil {
		t.Fatal(err)
	}
	assertTestFile(t, filepath.Join(root, "bin", "panel-api"), "new-bin", 0o755)
}

func TestApplyRecoversInterruptedTransactionBeforeReadingBundle(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "bin", "panel-api"), []byte("mixed-new"), 0o755)
	journal := filepath.Join(root, transactionDirectory)
	writeTestFile(t, filepath.Join(journal, "targets", "bin", "panel-api"), []byte("old-api"), 0o755)
	writeTestFile(t, filepath.Join(journal, "metadata.json"), []byte(
		`{"state":"applying","targets":[{"target":"bin/panel-api","exists":true}],"current_release_exists":false}`,
	), 0o600)

	if err := Apply(t.TempDir(), root, nil); err == nil {
		t.Fatal("expected missing manifest error after recovery")
	}
	assertTestFile(t, filepath.Join(root, "bin", "panel-api"), "old-api", 0o755)
	if _, err := os.Stat(journal); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy apply did not clean recovered journal: %v", err)
	}
}

func TestInstallUpdatesBinariesAndPortals(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "bin", "panel-api"), []byte("old-api"), 0o755)
	writeTestFile(t, filepath.Join(root, "share", "portals", "server", "index.html"), []byte("old-portal"), 0o644)

	artifacts := map[string]testArtifact{
		"panel-api": {
			target: "bin/panel-api", content: []byte("new-api"), mode: 0o755,
		},
		"server/index.html": {
			target: "share/portals/server/index.html", content: []byte("new-portal"), mode: 0o644,
		},
	}
	config, cleanup := newInstallConfig(t, root, artifacts)
	defer cleanup()
	runner := &recordingRunner{}

	status, err := Install(context.Background(), config, runner)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "installed" || status.InstalledRelease != "2.0.0" {
		t.Fatalf("unexpected status: %+v", status)
	}
	if status.LastCheckedAt == "" {
		t.Fatalf("install status did not record check time: %+v", status)
	}
	assertTestFile(t, filepath.Join(root, "bin", "panel-api"), "new-api", 0o755)
	assertTestFile(t, filepath.Join(root, "share", "portals", "server", "index.html"), "new-portal", 0o644)
	assertTestFile(t, filepath.Join(root, "current-release"), "2.0.0\n", 0o644)
	if !runner.contains("/bin/systemctl", "daemon-reload") ||
		!runner.contains("/usr/bin/curl", "-fsS", "http://127.0.0.1:18080/healthz") {
		t.Fatalf("fixed activation and health commands were not run: %v", runner.calls)
	}
}

func TestInstallRollsBackWhenHealthCheckFails(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "bin", "panel-api"), []byte("old-api"), 0o755)
	writeTestFile(t, filepath.Join(root, "current-release"), []byte("1.0.0\n"), 0o644)
	artifacts := map[string]testArtifact{
		"panel-api": {
			target: "bin/panel-api", content: []byte("bad-api"), mode: 0o755,
		},
		"account/index.html": {
			target: "share/portals/account/index.html", content: []byte("new-file"), mode: 0o644,
		},
	}
	config, cleanup := newInstallConfig(t, root, artifacts)
	defer cleanup()
	runner := &recordingRunner{failOnceOn: "http://127.0.0.1:18080/healthz"}

	status, err := Install(context.Background(), config, runner)
	if err == nil || !strings.Contains(err.Error(), "health") {
		t.Fatalf("expected health failure, got status=%+v err=%v", status, err)
	}
	assertTestFile(t, filepath.Join(root, "bin", "panel-api"), "old-api", 0o755)
	assertTestFile(t, filepath.Join(root, "current-release"), "1.0.0\n", 0o644)
	if _, statErr := os.Stat(filepath.Join(root, "share", "portals", "account", "index.html")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("new target survived rollback: %v", statErr)
	}
	if status.State != "rolled-back" {
		t.Fatalf("rollback status = %+v", status)
	}
}

func TestInstallLockRejectsConcurrentOperation(t *testing.T) {
	root := t.TempDir()
	statusPath := filepath.Join(t.TempDir(), "update-status.json")
	authoritative := []byte(`{"state":"installing","installed_release":"1.0.0"}`)
	writeTestFile(t, statusPath, authoritative, 0o600)
	config := Config{
		FeedURL: "https://updates.example.test", Channel: "stable",
		InstalledRelease: "1.0.0", InstallRoot: root,
		StatusPath: statusPath,
	}
	lock, err := acquireInstallLock(root)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()

	_, err = Install(context.Background(), config, &recordingRunner{})
	if err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("expected operation lock error, got %v", err)
	}
	got, readErr := os.ReadFile(statusPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != string(authoritative) {
		t.Fatalf("lock rejection overwrote authoritative status: %s", got)
	}
}

func TestCheckLockRejectionDoesNotContactNetworkOrOverwriteStatus(t *testing.T) {
	root := t.TempDir()
	statusPath := filepath.Join(t.TempDir(), "update-status.json")
	authoritative := []byte(`{"state":"installing","installed_release":"1.0.0"}`)
	writeTestFile(t, statusPath, authoritative, 0o600)
	var hits atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		http.Error(w, "should not be reached", http.StatusInternalServerError)
	}))
	defer server.Close()
	withHTTPClient(t, server.Client())
	lock, err := acquireInstallLock(root)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()

	_, err = Check(context.Background(), Config{
		FeedURL: server.URL, Channel: "stable", InstalledRelease: "1.0.0",
		InstallRoot: root, StatusPath: statusPath,
	})
	if err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("expected check lock rejection, got %v", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("check contacted feed %d times before acquiring lock", hits.Load())
	}
	got, readErr := os.ReadFile(statusPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != string(authoritative) {
		t.Fatalf("check lock rejection overwrote authoritative status: %s", got)
	}
}

func TestCheckRecoversServicesBeforeDeletingInterruptedJournal(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "bin", "panel-api"), []byte("interrupted-new"), 0o755)
	writeTestFile(t, filepath.Join(root, "current-release"), []byte("2.0.0\n"), 0o644)
	journal := filepath.Join(root, transactionDirectory)
	writeTestFile(t, filepath.Join(journal, "targets", "bin", "panel-api"), []byte("old-api"), 0o755)
	writeTestFile(t, filepath.Join(journal, "current-release"), []byte("1.0.0\n"), 0o644)
	writeTestFile(t, filepath.Join(journal, "metadata.json"), []byte(
		`{"state":"applying","targets":[{"target":"bin/panel-api","exists":true}],"current_release_exists":true}`,
	), 0o600)
	config, cleanup := newInstallConfig(t, root, map[string]testArtifact{
		"panel-api": {target: "bin/panel-api", content: []byte("release"), mode: 0o755},
	})
	defer cleanup()
	config.InstalledRelease = "2.0.0"
	runner := &recordingRunner{}
	config.RecoveryRunner = runner

	status, err := Check(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	if status.AvailableRelease != "2.0.0" {
		t.Fatalf("unexpected recovered check status: %+v", status)
	}
	if !runner.contains("/bin/systemctl", "restart", "panel-api", "panel-worker", "panel-agent", "nginx") ||
		!runner.contains("/usr/bin/curl", "-fsS", "http://127.0.0.1:18080/healthz") {
		t.Fatalf("recovery did not restart and health-check prior services: %v", runner.calls)
	}
	if _, err := os.Stat(journal); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("successful recovery left journal behind: %v", err)
	}
}

func TestCheckKeepsJournalWhenRecoveryHealthFails(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "bin", "panel-api"), []byte("interrupted-new"), 0o755)
	journal := filepath.Join(root, transactionDirectory)
	writeTestFile(t, filepath.Join(journal, "targets", "bin", "panel-api"), []byte("old-api"), 0o755)
	writeTestFile(t, filepath.Join(journal, "metadata.json"), []byte(
		`{"state":"applying","targets":[{"target":"bin/panel-api","exists":true}],"current_release_exists":false}`,
	), 0o600)
	runner := &recordingRunner{failOnceOn: "http://127.0.0.1:18080/healthz"}

	_, err := Check(context.Background(), Config{
		FeedURL: "https://updates.example.test", Channel: "stable",
		InstalledRelease: "2.0.0", InstallRoot: root,
		RecoveryRunner: runner,
	})
	if err == nil || !strings.Contains(err.Error(), "health") {
		t.Fatalf("expected recovery health failure, got %v", err)
	}
	assertTestFile(t, filepath.Join(root, "bin", "panel-api"), "old-api", 0o755)
	if _, statErr := os.Stat(journal); statErr != nil {
		t.Fatalf("failed recovery deleted journal: %v", statErr)
	}
}

func TestLegacyApplyAndRollbackShareInstallLock(t *testing.T) {
	root := t.TempDir()
	lock, err := acquireInstallLock(root)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()

	if err := Apply(t.TempDir(), root, nil); err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("legacy apply did not share advisory lock: %v", err)
	}
	if err := Rollback(root); err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("legacy rollback did not share advisory lock: %v", err)
	}
}

func TestInstallLockUsesCanonicalInstallRootOnly(t *testing.T) {
	root := t.TempDir()
	lock, err := acquireInstallLock(root)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	equivalent := filepath.Join(root, "..", filepath.Base(root))
	if _, err := acquireInstallLock(equivalent); err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("equivalent install root did not contend on the same lock: %v", err)
	}
}

func TestInstallRecoversInterruptedTransactionBeforeCheckingFeed(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "bin", "panel-api"), []byte("mixed-new"), 0o755)
	writeTestFile(t, filepath.Join(root, "current-release"), []byte("2.0.0\n"), 0o644)
	journal := filepath.Join(root, ".update-transaction")
	writeTestFile(t, filepath.Join(journal, "targets", "bin", "panel-api"), []byte("old-api"), 0o755)
	writeTestFile(t, filepath.Join(journal, "current-release"), []byte("1.0.0\n"), 0o644)
	writeTestFile(t, filepath.Join(journal, "metadata.json"), []byte(
		`{"state":"applying","targets":[{"target":"bin/panel-api","exists":true}],"current_release_exists":true}`,
	), 0o600)

	_, err := Install(context.Background(), Config{
		FeedURL: "http://invalid.example.test", Channel: "stable",
		InstalledRelease: "1.0.0", InstallRoot: root,
	}, &recordingRunner{})
	if err == nil {
		t.Fatal("expected feed validation failure after recovery")
	}
	assertTestFile(t, filepath.Join(root, "bin", "panel-api"), "old-api", 0o755)
	assertTestFile(t, filepath.Join(root, "current-release"), "1.0.0\n", 0o644)
	if _, statErr := os.Stat(journal); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("recovered journal remains: %v", statErr)
	}
}

func TestInstallReloadsRecoveredCurrentReleaseBeforeVersionCheck(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "bin", "panel-api"), []byte("interrupted-new"), 0o755)
	writeTestFile(t, filepath.Join(root, "current-release"), []byte("2.0.0\n"), 0o644)
	journal := filepath.Join(root, transactionDirectory)
	writeTestFile(t, filepath.Join(journal, "targets", "bin", "panel-api"), []byte("old-api"), 0o755)
	writeTestFile(t, filepath.Join(journal, "current-release"), []byte("1.0.0\n"), 0o644)
	writeTestFile(t, filepath.Join(journal, "metadata.json"), []byte(
		`{"state":"applying","targets":[{"target":"bin/panel-api","exists":true}],"current_release_exists":true}`,
	), 0o600)
	config, cleanup := newInstallConfig(t, root, map[string]testArtifact{
		"panel-api": {target: "bin/panel-api", content: []byte("complete-new"), mode: 0o755},
	})
	defer cleanup()
	config.InstalledRelease = "2.0.0"

	status, err := Install(context.Background(), config, &recordingRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if status.InstalledRelease != "2.0.0" {
		t.Fatalf("unexpected installed release: %+v", status)
	}
	assertTestFile(t, filepath.Join(root, "bin", "panel-api"), "complete-new", 0o755)
}

func TestInstallRejectsSymlinkedInstallRootAncestor(t *testing.T) {
	realParent := t.TempDir()
	if err := os.Mkdir(filepath.Join(realParent, "panel"), 0o755); err != nil {
		t.Fatal(err)
	}
	aliasParent := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(realParent, aliasParent); err != nil {
		t.Fatal(err)
	}

	_, err := Install(context.Background(), Config{
		FeedURL: "https://updates.example.test", Channel: "stable",
		InstalledRelease: "1.0.0", InstallRoot: filepath.Join(aliasParent, "panel"),
	}, &recordingRunner{})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "symlink") {
		t.Fatalf("expected symlinked ancestor rejection, got %v", err)
	}
}

func TestApplyAndRollbackRejectSymlinkedPaths(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	bundle := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	writeTestFile(t, outside, []byte("payload"), 0o755)
	if err := os.Symlink(outside, filepath.Join(bundle, "panel-api")); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("payload"))
	manifest := &Manifest{
		Release: "1.2.0", Channel: "stable",
		Files: map[string]string{"panel-api": hex.EncodeToString(sum[:])},
	}
	if err := Sign(manifest, priv); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(bundle, "manifest.json"), raw, 0o644)
	if err := Apply(bundle, t.TempDir(), pub); err == nil || !strings.Contains(strings.ToLower(err.Error()), "symlink") {
		t.Fatalf("apply followed artifact symlink: %v", err)
	}

	realRoot := t.TempDir()
	alias := filepath.Join(t.TempDir(), "panel")
	if err := os.Symlink(realRoot, alias); err != nil {
		t.Fatal(err)
	}
	if err := Rollback(alias); err == nil || !strings.Contains(strings.ToLower(err.Error()), "symlink") {
		t.Fatalf("rollback accepted symlinked root: %v", err)
	}
}

func TestInstallRejectsSymlinkTargetWithoutLeavingTransaction(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	writeTestFile(t, outside, []byte("outside"), 0o755)
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "bin", "panel-api")); err != nil {
		t.Fatal(err)
	}
	config, cleanup := newInstallConfig(t, root, map[string]testArtifact{
		"panel-api": {target: "bin/panel-api", content: []byte("new-api"), mode: 0o755},
	})
	defer cleanup()

	if _, err := Install(context.Background(), config, &recordingRunner{}); err == nil ||
		!strings.Contains(strings.ToLower(err.Error()), "symlink") {
		t.Fatalf("expected symlink target rejection, got %v", err)
	}
	assertTestFile(t, outside, "outside", 0o755)
	if _, err := os.Stat(filepath.Join(root, transactionDirectory)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed pre-apply transaction was not cleaned up: %v", err)
	}
}

func TestInstallRollsBackWhenSuccessStatusCannotPersist(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "bin", "panel-api"), []byte("old-api"), 0o755)
	writeTestFile(t, filepath.Join(root, "current-release"), []byte("1.0.0\n"), 0o644)
	config, cleanup := newInstallConfig(t, root, map[string]testArtifact{
		"panel-api": {target: "bin/panel-api", content: []byte("new-api"), mode: 0o755},
	})
	defer cleanup()
	config.StatusPath = filepath.Join(t.TempDir(), "status-as-directory")
	if err := os.Mkdir(config.StatusPath, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(config.StatusPath, "keep"), []byte("not empty"), 0o600)

	status, err := Install(context.Background(), config, &recordingRunner{})
	if err == nil {
		t.Fatal("expected final status persistence failure")
	}
	if status.State != "rolled-back" {
		t.Fatalf("status persistence failure did not roll back: %+v, %v", status, err)
	}
	assertTestFile(t, filepath.Join(root, "bin", "panel-api"), "old-api", 0o755)
	assertTestFile(t, filepath.Join(root, "current-release"), "1.0.0\n", 0o644)
}

func TestInstallUsesFreshContextForRecoveryAfterCancellation(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "bin", "panel-api"), []byte("old-api"), 0o755)
	config, cleanup := newInstallConfig(t, root, map[string]testArtifact{
		"panel-api": {target: "bin/panel-api", content: []byte("new-api"), mode: 0o755},
	})
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	runner := &cancelThenRecoverRunner{cancel: cancel}

	status, err := Install(ctx, config, runner)
	if err == nil {
		t.Fatal("expected canceled activation")
	}
	if status.State != "rolled-back" {
		t.Fatalf("fresh recovery context was not used: status=%+v err=%v", status, err)
	}
	assertTestFile(t, filepath.Join(root, "bin", "panel-api"), "old-api", 0o755)
}

func TestInstallModesIgnoreRestrictiveUmask(t *testing.T) {
	root := t.TempDir()
	config, cleanup := newInstallConfig(t, root, map[string]testArtifact{
		"panel-api": {
			target: "bin/panel-api", content: []byte("executable"), mode: 0o755,
		},
		"server/index.html": {
			target: "share/portals/server/index.html", content: []byte("readonly"), mode: 0o644,
		},
	})
	defer cleanup()
	previousUmask := unix.Umask(0o077)
	defer unix.Umask(previousUmask)

	if _, err := Install(context.Background(), config, &recordingRunner{}); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{
		"bin", "share", "share/portals", "share/portals/server",
	} {
		assertTestMode(t, filepath.Join(root, directory), 0o755)
	}
	assertTestFile(t, filepath.Join(root, "bin", "panel-api"), "executable", 0o755)
	assertTestFile(t, filepath.Join(root, "share", "portals", "server", "index.html"), "readonly", 0o644)
}

func TestRollbackModesIgnoreRestrictiveUmask(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "bin", "panel-api"), []byte("old-api"), 0o755)
	writeTestFile(t, filepath.Join(root, "share", "portals", "server", "index.html"), []byte("old-portal"), 0o644)
	config, cleanup := newInstallConfig(t, root, map[string]testArtifact{
		"panel-api": {
			target: "bin/panel-api", content: []byte("bad-api"), mode: 0o755,
		},
		"server/index.html": {
			target: "share/portals/server/index.html", content: []byte("bad-portal"), mode: 0o644,
		},
	})
	defer cleanup()
	previousUmask := unix.Umask(0o077)
	defer unix.Umask(previousUmask)

	if _, err := Install(
		context.Background(),
		config,
		&recordingRunner{failOnceOn: "http://127.0.0.1:18080/healthz"},
	); err == nil {
		t.Fatal("expected health failure")
	}
	assertTestFile(t, filepath.Join(root, "bin", "panel-api"), "old-api", 0o755)
	assertTestFile(t, filepath.Join(root, "share", "portals", "server", "index.html"), "old-portal", 0o644)
}

func TestSecureDirectoryModesIgnoreRestrictiveUmask(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "created", "panel")
	previousUmask := unix.Umask(0o077)
	defer unix.Umask(previousUmask)
	if err := createSecureRoot(rootPath, "test root"); err != nil {
		t.Fatal(err)
	}
	assertTestMode(t, filepath.Dir(rootPath), 0o755)
	assertTestMode(t, rootPath, 0o755)
	root, err := openSecureRoot(rootPath, "test root")
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := root.mkdirAll("private/one/two", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := root.copyReaderAtomic(
		bytes.NewReader([]byte("leaf")),
		"public/one/two/file",
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	if err := root.copyReaderAtomicWithDirMode(
		bytes.NewReader([]byte("private-leaf")),
		"staging/one/two/file",
		0o644,
		0o700,
	); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{"private", "private/one", "private/one/two"} {
		assertTestMode(t, filepath.Join(rootPath, directory), 0o700)
	}
	for _, directory := range []string{"public", "public/one", "public/one/two"} {
		assertTestMode(t, filepath.Join(rootPath, directory), 0o755)
	}
	for _, directory := range []string{"staging", "staging/one", "staging/one/two"} {
		assertTestMode(t, filepath.Join(rootPath, directory), 0o700)
	}
	assertTestFile(t, filepath.Join(rootPath, "public", "one", "two", "file"), "leaf", 0o644)
	assertTestFile(t, filepath.Join(rootPath, "staging", "one", "two", "file"), "private-leaf", 0o644)
}

type testArtifact struct {
	target  string
	content []byte
	mode    uint32
}

type recordingRunner struct {
	calls      [][]string
	failOnceOn string
	failed     bool
}

type cancelThenRecoverRunner struct {
	cancel          context.CancelFunc
	calls           int
	recoveryWasLive bool
}

func (r *cancelThenRecoverRunner) Run(ctx context.Context, _ string, _ ...string) error {
	r.calls++
	if r.calls == 1 {
		r.cancel()
		return context.Canceled
	}
	if ctx.Err() != nil {
		return fmt.Errorf("recovery context is canceled: %w", ctx.Err())
	}
	r.recoveryWasLive = true
	return nil
}

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) error {
	call := append([]string{name}, args...)
	r.calls = append(r.calls, call)
	if !r.failed && r.failOnceOn != "" && strings.Contains(strings.Join(call, " "), r.failOnceOn) {
		r.failed = true
		return fmt.Errorf("health check failed")
	}
	return nil
}

func (r *recordingRunner) contains(want ...string) bool {
	for _, call := range r.calls {
		if strings.Join(call, "\x00") == strings.Join(want, "\x00") {
			return true
		}
	}
	return false
}

func newInstallConfig(t *testing.T, root string, artifacts map[string]testArtifact) (Config, func()) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifest := &Manifest{
		Release: "2.0.0", Channel: "stable", MinimumRelease: "1.0.0",
	}
	for artifactPath, artifact := range artifacts {
		sum := sha256.Sum256(artifact.content)
		manifest.Artifacts = append(manifest.Artifacts, Artifact{
			Path: artifactPath, Target: artifact.target, Size: int64(len(artifact.content)),
			SHA256: hex.EncodeToString(sum[:]), Mode: artifact.mode,
		})
	}
	if err := Sign(manifest, priv); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/stable/manifest.json":
			_, _ = w.Write(raw)
		case strings.HasPrefix(r.URL.Path, "/stable/2.0.0/"):
			artifactPath := strings.TrimPrefix(r.URL.Path, "/stable/2.0.0/")
			artifact, ok := artifacts[artifactPath]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(artifact.content)
		default:
			http.NotFound(w, r)
		}
	}))
	previous := feedClientFactory
	feedClientFactory = func(base *url.URL, timeout time.Duration) *http.Client {
		client := server.Client()
		client.Timeout = timeout
		client.CheckRedirect = sameHostRedirectPolicy(base)
		return client
	}
	return Config{
			FeedURL: server.URL, Channel: "stable", InstalledRelease: "1.0.0",
			PublicKey: pub, InstallRoot: root,
			StatusPath: filepath.Join(t.TempDir(), "update-status.json"),
			Automatic:  true,
		}, func() {
			feedClientFactory = previous
			server.Close()
		}
}

func writeTestFile(t *testing.T, path string, content []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatal(err)
	}
}

func assertTestFile(t *testing.T, path, want string, mode os.FileMode) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != want {
		t.Fatalf("%s = %q, want %q", path, raw, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != mode {
		t.Fatalf("%s mode = %o, want %o", path, info.Mode().Perm(), mode)
	}
}

func assertTestMode(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != mode {
		t.Fatalf("%s mode = %o, want %o", path, info.Mode().Perm(), mode)
	}
}
