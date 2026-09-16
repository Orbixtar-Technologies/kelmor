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
	if len(addrs) < 1 || addrs[0] != "127.0.0.1" {
		t.Fatalf("%v", addrs)
	}
	if isAssignedIPv4("203.0.113.9") {
		if len(addrs) != 2 || addrs[1] != "203.0.113.9" {
			t.Fatalf("%v", addrs)
		}
	} else if len(addrs) != 2 || addrs[1] != primaryInterfaceIPv4() {
		t.Fatalf("expected NIC fallback, got %v", addrs)
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

func TestPublicIPv4SkipsPrivateEnv(t *testing.T) {
	t.Setenv("PANEL_PUBLIC_IPV4", "172.21.76.48")
	t.Setenv("PANEL_PUBLIC_ENV", t.TempDir()+"/missing.env")
	orig := lookupExternalIPv4
	t.Cleanup(func() { lookupExternalIPv4 = orig })
	lookupExternalIPv4 = func() string { return "47.85.187.178" }
	got := PublicIPv4()
	if IsPrivateIPv4String(got) {
		t.Fatalf("still private %s", got)
	}
}

func TestIsPrivateIPv4(t *testing.T) {
	if !IsPrivateIPv4String("172.21.76.48") || !IsPrivateIPv4String("127.0.0.1") {
		t.Fatal("expected private")
	}
	if IsPrivateIPv4String("47.85.187.178") || IsPrivateIPv4String("203.0.113.50") {
		t.Fatal("expected public")
	}
}

func TestRewritePrivateIP4Tokens(t *testing.T) {
	got := RewritePrivateIP4Tokens("v=spf1 a mx ip4:172.21.76.48 ~all", "47.85.187.178")
	if got != "v=spf1 a mx ip4:47.85.187.178 ~all" {
		t.Fatalf("%q", got)
	}
}
