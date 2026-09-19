package firewall

import (
	"strings"
	"testing"
)

func TestRulesIncludesHostingAndExtra(t *testing.T) {
	body := Rules([]int{26054, 22})
	if !contains(body, "table inet panel") {
		t.Fatal(body)
	}
	if !contains(body, "26054") || !contains(body, "443") || !contains(body, "21") {
		t.Fatal(body)
	}
	if !contains(body, "40000-40100") {
		t.Fatal("passive FTP range")
	}
	if !contains(body, "policy drop") {
		t.Fatal("must be a real drop policy")
	}
}

func TestRulesIsolatePowerDNSManagement(t *testing.T) {
	body := Rules(nil)
	if !contains(body, "chain output") {
		t.Fatal("missing output filter")
	}
	if !contains(body, "8081") {
		t.Fatal("missing PowerDNS management port")
	}
	if !contains(body, "ip6 daddr ::1") {
		t.Fatal("missing IPv6 loopback management filter")
	}
	if !contains(body, "skuid") {
		t.Fatal("missing identity-based output filter")
	}
}

func TestRulesWithAccessDropsDeniedCIDR(t *testing.T) {
	body := RulesWithAccess(nil, []string{"203.0.113.10"}, []string{"198.51.100.0/24"})
	if !contains(body, "ip saddr 198.51.100.0/24 drop") {
		t.Fatal(body)
	}
	if !contains(body, "ip saddr 203.0.113.10/32 accept") {
		t.Fatal(body)
	}
}

func TestSplitHexAddr(t *testing.T) {
	host, port, err := splitHexAddr("0100007F:4652")
	if err != nil || port != 0x4652 || host != "0100007F" {
		t.Fatalf("%s %d %v", host, port, err)
	}
	if !isLoopbackHex(host) {
		t.Fatal("127.0.0.1")
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
