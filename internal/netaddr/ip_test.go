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
	if len(addrs) != 2 || addrs[0] != "127.0.0.1" || addrs[1] != "203.0.113.9" {
		t.Fatalf("%v", addrs)
	}
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
