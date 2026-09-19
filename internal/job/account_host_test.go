package job

import (
	"testing"

	"github.com/hosting-panel/panel/internal/store"
)

func TestPosixShellAndPublishAddresses(t *testing.T) {
	if posixShell("sftp-only") != "/usr/sbin/rssh" {
		t.Fatalf("sftp shell %s", posixShell("sftp-only"))
	}
	if posixShell("jailed") != "/bin/bash" {
		t.Fatalf("jailed shell %s", posixShell("jailed"))
	}
	if posixShell("nologin") != "/usr/sbin/nologin" {
		t.Fatalf("nologin shell %s", posixShell("nologin"))
	}
	acc := &store.Account{IPAddress: "203.0.113.20"}
	if accountPublishIPv4(acc) != "203.0.113.20" {
		t.Fatalf("dedicated IPv4 %s", accountPublishIPv4(acc))
	}
	if accountPublishIPv6(acc) != "" {
		t.Fatalf("IPv4 account must not publish AAAA")
	}
	v6 := &store.Account{IPAddress: "2001:db8::10"}
	if accountPublishIPv6(v6) != "2001:db8::10" {
		t.Fatalf("dedicated IPv6 %s", accountPublishIPv6(v6))
	}
	if restorePathPrefix("../etc/passwd") != "" {
		t.Fatal("traversal prefix must be rejected")
	}
	if restorePathPrefix("public_html/app") != "public_html/app" {
		t.Fatalf("prefix %s", restorePathPrefix("public_html/app"))
	}
}
