package job

import (
	"testing"

	"github.com/hosting-panel/panel/internal/store"
)

func TestAccountShellPath(t *testing.T) {
	cases := []struct {
		class string
		want  string
	}{
		{"", "/usr/sbin/nologin"},
		{"nologin", "/usr/sbin/nologin"},
		{"sftp-only", "/usr/sbin/nologin"},
		{"jailed", "/usr/sbin/rssh"},
	}
	for _, tc := range cases {
		if got := accountShellPath(tc.class); got != tc.want {
			t.Fatalf("accountShellPath(%q)=%q want %q", tc.class, got, tc.want)
		}
	}
}

func TestAccountPublishAddressesPreferDedicated(t *testing.T) {
	t.Setenv("PANEL_PUBLIC_IPV4", "203.0.113.10")
	acc := &store.Account{IPAddress: "198.51.100.20"}
	if got := accountPublishIPv4(acc); got != "198.51.100.20" {
		t.Fatalf("dedicated IPv4: %q", got)
	}
	acc.IPAddress = "2001:db8::20"
	if got := accountPublishIPv4(acc); got != "203.0.113.10" {
		t.Fatalf("IPv6 account still publishes shared A: %q", got)
	}
	if got := accountPublishIPv6(acc); got != "2001:db8::20" {
		t.Fatalf("dedicated IPv6: %q", got)
	}
	acc.IPAddress = ""
	if got := accountPublishIPv6(acc); got != "" {
		t.Fatalf("empty IPv6: %q", got)
	}
}

func TestRestorePathPrefix(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"public_html", "public_html"},
		{"/public_html/assets", "public_html/assets"},
		{"../etc", ""},
		{"", ""},
		{".", ""},
	}
	for _, tc := range cases {
		if got := restorePathPrefix(tc.in); got != tc.want {
			t.Fatalf("restorePathPrefix(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestManagedDNSName(t *testing.T) {
	if !managedDNSName("@") || !managedDNSName("www") || !managedDNSName("mail") {
		t.Fatal("expected managed apex and service names")
	}
	if managedDNSName("cdn") || managedDNSName("shop") {
		t.Fatal("custom names must stay unmanaged")
	}
}
