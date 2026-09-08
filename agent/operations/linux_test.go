package operations

import "testing"

func TestSetLinuxPasswordRejectsUnsafe(t *testing.T) {
	h := &Host{Root: "/tmp/panel-linux-test"}
	if _, err := h.setLinuxPassword("acme42", "short"); err == nil {
		t.Fatal("short password")
	}
	if _, err := h.setLinuxPassword("acme42", "bad:colon!!"); err == nil {
		t.Fatal("colon")
	}
	res, err := h.setLinuxPassword("acme42", "SftpPass!2026")
	if err != nil || !res.OK {
		t.Fatalf("%v %#v", err, res)
	}
}

func TestHostingDirModeKeepsTenantsPrivate(t *testing.T) {
	if hostingDirMode("/home/acme/public_html") != 0o750 {
		t.Fatal("public_html must not be world-readable")
	}
	if hostingDirMode("/home/acme/backups") != 0o750 {
		t.Fatal("private dirs stay 0750")
	}
}
