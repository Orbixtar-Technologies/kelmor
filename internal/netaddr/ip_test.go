package netaddr

import (
	"net"
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

func TestPublicIPv4RejectsNonV4(t *testing.T) {
	t.Setenv("PANEL_PUBLIC_IPV4", "not-an-ip")
	if got := PublicIPv4(); net.ParseIP(got) == nil {
		t.Fatalf("fallback %s", got)
	}
}
