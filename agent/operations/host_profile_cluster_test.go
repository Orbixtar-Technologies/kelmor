package operations

import (
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
