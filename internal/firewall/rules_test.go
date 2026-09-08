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
