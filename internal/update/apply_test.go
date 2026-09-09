package update

import (
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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	config := Config{
		FeedURL: "https://updates.example.test", Channel: "stable",
		InstalledRelease: "1.0.0", InstallRoot: root,
		StatusPath: filepath.Join(t.TempDir(), "update-status.json"),
	}
	lock := installLockPath(config)
	writeTestFile(t, lock, []byte("other updater\n"), 0o600)

	_, err := Install(context.Background(), config, &recordingRunner{})
	if err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("expected operation lock error, got %v", err)
	}
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
	previous := http.DefaultClient
	http.DefaultClient = server.Client()
	return Config{
			FeedURL: server.URL, Channel: "stable", InstalledRelease: "1.0.0",
			PublicKey: pub, InstallRoot: root,
			StatusPath: filepath.Join(t.TempDir(), "update-status.json"),
			Automatic:  true,
		}, func() {
			http.DefaultClient = previous
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
