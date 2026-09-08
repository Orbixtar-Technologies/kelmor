package operations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyFTPUsersWritesMaps(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	home := filepath.Join(root, "home", "acme42", "public_html")
	if err := os.MkdirAll(home, 0o750); err != nil {
		t.Fatal(err)
	}
	res, err := h.applyFTPUsers([]FTPUser{{
		Username:     "acmeftp",
		PasswordHash: "$6$rounds=5000$saltsalt$0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0",
		GuestUser:    "acme42",
		LocalRoot:    "/home/acme42/public_html",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("%+v", res)
	}
	body, err := os.ReadFile(filepath.Join(root, "var/lib/panel/ftp/passwd"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "acmeftp:$6$") {
		t.Fatalf("passwd %s", body)
	}
	conf, err := os.ReadFile(filepath.Join(root, "var/lib/panel/ftp/user_conf/acmeftp"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(conf), "guest_username=acme42") || !strings.Contains(string(conf), "local_root=/home/acme42/public_html") {
		t.Fatalf("conf %s", conf)
	}
	if _, err := h.applyFTPUsers(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "var/lib/panel/ftp/user_conf/acmeftp")); err == nil {
		t.Fatal("stale user_conf must be removed")
	}
}

func TestApplyFTPUsersRejectsEscape(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	_, err := h.applyFTPUsers([]FTPUser{{
		Username:     "acmeftp",
		PasswordHash: "$6$x$y",
		GuestUser:    "acme42",
		LocalRoot:    "/home/other/public_html",
	}})
	if err == nil {
		t.Fatal("cross-account root")
	}
}
