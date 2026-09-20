package operations

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyServerProfileWritesHostFile(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if err := h.applyServerProfile("fax"); err == nil {
		t.Fatal("expected invalid profile")
	}
	res, err := h.applyHostConfig(HostConfigSpec{Profile: "mail"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Message, "server-profile") {
		t.Fatalf("applied: %s", res.Message)
	}
	got, err := os.ReadFile(filepath.Join(h.Root, "etc/panel/server-profile"))
	if err != nil || strings.TrimSpace(string(got)) != "mail" {
		t.Fatalf("profile file: %s %v", got, err)
	}
}

func TestWriteClusterMembershipRejectsHostilePeers(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if err := h.writeClusterMembership([]string{"https://peer.test; rm -rf /"}); err == nil {
		t.Fatal("expected hostile peer rejection")
	}
	if _, err := h.applyHostConfig(HostConfigSpec{
		WriteCluster: true,
		ClusterPeers: []string{"https://peer.example.test:2087"},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(h.Root, "etc/panel/cluster.json"))
	if err != nil || !strings.Contains(string(got), "peer.example.test") {
		t.Fatalf("cluster file: %s %v", got, err)
	}
}

func TestClusterSnapshotRoundTrip(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if _, err := h.writeClusterSnapshot([]byte("not-json")); err == nil {
		t.Fatal("expected JSON validation")
	}
	payload := []byte(`{"packages":[{"name":"basic"}]}` + "\n")
	if _, err := h.writeClusterSnapshot(payload); err != nil {
		t.Fatal(err)
	}
	got, err := h.readClusterSnapshot()
	if err != nil || !strings.Contains(string(got), "basic") {
		t.Fatalf("snapshot: %s %v", got, err)
	}
}

func TestProbeClusterPeersHitsHealthz(t *testing.T) {
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(peer.Close)
	h := &Host{Root: t.TempDir()}
	raw, err := h.probeClusterPeers([]string{peer.URL})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := raw.(map[string]any)
	items, _ := got["items"].([]map[string]any)
	if len(items) != 1 || items[0]["ok"] != true {
		t.Fatalf("probe: %#v", raw)
	}
	if _, err := h.probeClusterPeers([]string{"https://evil.test;rm"}); err == nil {
		t.Fatal("expected hostile URL rejection")
	}
}

func TestApplyClusterSnapshotWritesLocalAndPostsPeers(t *testing.T) {
	var gotAuth string
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/server/cluster/snapshot/import" {
			http.NotFound(w, r)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(peer.Close)
	h := &Host{Root: t.TempDir()}
	raw, err := h.applyClusterSnapshot([]string{peer.URL}, []byte(`{"packages":[{"name":"shared"}]}`), "peer-token")
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer peer-token" {
		t.Fatalf("auth: %q", gotAuth)
	}
	got, err := h.readClusterSnapshot()
	if err != nil || !strings.Contains(string(got), "shared") {
		t.Fatalf("local snapshot: %s %v", got, err)
	}
	payload, _ := raw.(map[string]any)
	items, _ := payload["items"].([]map[string]any)
	if len(items) != 1 || items[0]["ok"] != true {
		t.Fatalf("apply: %#v", raw)
	}
	if _, err := h.applyClusterSnapshot([]string{"https://evil.test;rm"}, []byte(`{}`), ""); err == nil {
		t.Fatal("expected hostile URL rejection")
	}
}

func TestWriteRemoteAccessKeyPersistsHostFile(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if _, err := h.writeRemoteAccessKey(RemoteAccessRecord{Prefix: "nope", Hash: strings.Repeat("a", 64)}); err == nil {
		t.Fatal("expected prefix rejection")
	}
	res, err := h.writeRemoteAccessKey(RemoteAccessRecord{
		Prefix: "hp_remote_ab", Hash: strings.Repeat("ab", 32), CreatedAt: "2099-01-01T00:00:00Z",
	})
	if err != nil || !res.OK {
		t.Fatalf("write: %v %v", res, err)
	}
	got, err := os.ReadFile(filepath.Join(h.Root, "etc/panel/remote-access-key"))
	if err != nil || !strings.Contains(string(got), "hp_remote_ab") {
		t.Fatalf("file: %s %v", got, err)
	}
}

func TestProfileServiceUnitsNeverTouchManagement(t *testing.T) {
	for _, profile := range []string{"standard", "mail", "dns"} {
		enable, disable := profileServiceUnits(profile)
		for _, unit := range append(append([]string{}, enable...), disable...) {
			if unit == "nginx" || strings.HasPrefix(unit, "panel-") || unit == "postgresql" {
				t.Fatalf("%s listed management unit %s", profile, unit)
			}
		}
	}
	_, disable := profileServiceUnits("dns")
	joined := strings.Join(disable, ",")
	if !strings.Contains(joined, "postfix") || !strings.Contains(joined, "php8.3-fpm") {
		t.Fatalf("dns disable set: %s", joined)
	}
}
