package phases

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
