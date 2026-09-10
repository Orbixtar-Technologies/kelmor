package acme

import "testing"

func TestHostnamesForPortal(t *testing.T) {
	got := HostnamesForPortal("lab.kelmor.host")
	want := map[string]bool{
		"lab.kelmor.host": true, "www.lab.kelmor.host": true, "kelmor.host": true,
	}
	if len(got) != len(want) {
		t.Fatalf("portal names: %v", got)
	}
	for _, name := range got {
		if !want[name] {
			t.Fatalf("unexpected %q in %v", name, got)
		}
	}
	if HostnamesForPortal("localhost") != nil {
		t.Fatal("localhost excluded")
	}
}

func TestHostnamesForSitePrimary(t *testing.T) {
	got := HostnamesForSite("example.com", []string{"alias.example.net"}, true)
	want := map[string]bool{
		"example.com": true, "alias.example.net": true,
		"www.example.com": true, "webmail.example.com": true,
		"phpmyadmin.example.com": true, "mail.example.com": true,
	}
	if len(got) != len(want) {
		t.Fatalf("count %d vs %d: %v", len(got), len(want), got)
	}
	for _, name := range got {
		if !want[name] {
			t.Fatalf("unexpected %q in %v", name, got)
		}
	}
}

func TestHostnamesForSiteAddon(t *testing.T) {
	got := HostnamesForSite("shop.example.com", nil, false)
	if len(got) != 1 || got[0] != "shop.example.com" {
		t.Fatalf("addon names: %v", got)
	}
}
