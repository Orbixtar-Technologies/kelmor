package netaddr

import (
	"net"
	"os"
	"testing"
)

func TestPublicIPv4HonorsEnv(t *testing.T) {
	t.Setenv("PANEL_PUBLIC_IPV4", "203.0.113.9")
	if got := PublicIPv4(); got != "203.0.113.9" {
		t.Fatalf("got %s", got)
	}
	addrs := DNSListenIPv4()
	if len(addrs) != 1 || addrs[0] != "0.0.0.0" {
		t.Fatalf("unbound public ip should listen 0.0.0.0, got %v", addrs)
	}
}

func TestDNSListenIPv4UsesLocalPublicAddress(t *testing.T) {
	local := firstLocalIPv4(t)
	t.Setenv("PANEL_PUBLIC_IPV4", local)
	addrs := DNSListenIPv4()
	if len(addrs) != 2 || addrs[0] != "127.0.0.1" || addrs[1] != local {
		t.Fatalf("%v", addrs)
	}
	if !AddressIsLocal(local) {
		t.Fatal("expected local address")
	}
	if AddressIsLocal("203.0.113.9") {
		t.Fatal("documentation net must not be local")
	}
}

func firstLocalIPv4(t *testing.T) string {
	t.Helper()
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range addrs {
		n, ok := a.(*net.IPNet)
		if !ok || n.IP == nil || n.IP.To4() == nil || n.IP.IsLoopback() {
			continue
		}
		return n.IP.To4().String()
	}
	t.Skip("no non-loopback IPv4 on this host")
	return ""
}

func TestReadPublicEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/public.env"
	if err := os.WriteFile(path, []byte("PANEL_PUBLIC_IPV4=198.51.100.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PANEL_PUBLIC_ENV", path)
	if got := readPublicEnvFile(); got != "198.51.100.4" {
		t.Fatalf("got %s", got)
	}
}

func TestPublicIPv4RejectsNonV4(t *testing.T) {
	t.Setenv("PANEL_PUBLIC_IPV4", "not-an-ip")
	if got := PublicIPv4(); net.ParseIP(got) == nil {
		t.Fatalf("fallback %s", got)
	}
}
