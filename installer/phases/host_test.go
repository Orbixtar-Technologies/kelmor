package phases

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMailRestartPlanPrefersSystemd(t *testing.T) {
	plan := mailRestartPlan(true)
	joined := ""
	for _, args := range plan {
		joined += strings.Join(args, " ") + "\n"
	}
	if !strings.Contains(joined, "systemctl restart dovecot") {
		t.Fatalf("systemd mail restart must use the unit: %q", joined)
	}
	if strings.Contains(joined, "/usr/sbin/dovecot") && !strings.Contains(joined, "systemctl") {
		t.Fatal("must not hand-start dovecot when systemd is PID 1")
	}
	legacy := mailRestartPlan(false)
	legacyJoined := ""
	for _, args := range legacy {
		legacyJoined += strings.Join(args, " ") + "\n"
	}
	if !strings.Contains(legacyJoined, "/usr/sbin/dovecot") {
		t.Fatalf("non-systemd fallback missing: %q", legacyJoined)
	}
}

func TestSystemdUnitForHost(t *testing.T) {
	if got := systemdUnitForHost("pdns_server"); got != "pdns" {
		t.Fatalf("pdns %q", got)
	}
	if got := systemdUnitForHost("dovecot"); got != "dovecot" {
		t.Fatalf("dovecot %q", got)
	}
	if got := systemdUnitForHost("unknown"); got != "" {
		t.Fatalf("unknown %q", got)
	}
}

func TestHostServicesStartPowerDNSWithConfigDir(t *testing.T) {
	var pdns *hostService
	for i := range hostServices() {
		s := hostServices()[i]
		if s.Comm == "pdns_server" {
			pdns = &s
			break
		}
	}
	if pdns == nil {
		t.Fatal("pdns_server missing from host services")
	}
	if pdns.Listen != "127.0.0.1:53" {
		t.Fatalf("listen %s", pdns.Listen)
	}
	joined := ""
	for _, a := range pdns.Args {
		joined += " " + a
	}
	if !contains(joined, "--config-dir=/etc/powerdns") || !contains(joined, "--daemon=yes") {
		t.Fatalf("pdns args %v", pdns.Args)
	}
	need := map[string]string{"mysqld": "127.0.0.1:3306", "postgres": "127.0.0.1:5432"}
	for _, s := range hostServices() {
		if want, ok := need[s.Comm]; ok {
			if s.Listen != want {
				t.Fatalf("%s listen %s", s.Comm, s.Listen)
			}
			delete(need, s.Comm)
		}
	}
	if len(need) != 0 {
		t.Fatalf("missing host services %v", need)
	}
}

func TestLoadEnvPairsReadsPanelKeys(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "backup-s3.env")
	if err := os.WriteFile(p, []byte("# comment\nPANEL_S3_BUCKET=panel\nIGNORE=1\n\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	got := loadEnvPairs(p)
	if len(got) != 1 || got[0] != "PANEL_S3_BUCKET=panel" {
		t.Fatalf("%v", got)
	}
}

func TestPid1IsSystemdMatchesProcComm(t *testing.T) {
	b, err := os.ReadFile("/proc/1/comm")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSpace(string(b)) == "systemd"
	if got := pid1IsSystemd(); got != want {
		t.Fatalf("pid1IsSystemd()=%v want %v (pid1=%q)", got, want, strings.TrimSpace(string(b)))
	}
}

func TestWaitListenAcceptsOpenPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if err := waitListen(ln.Addr().String(), time.Second); err != nil {
		t.Fatal(err)
	}
	if err := waitListen("127.0.0.1:1", 200*time.Millisecond); err == nil {
		t.Fatal("expected timeout")
	}
}
