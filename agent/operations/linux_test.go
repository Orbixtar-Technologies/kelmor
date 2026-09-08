package operations

import "testing"

func TestHostingDirModeKeepsTenantsPrivate(t *testing.T) {
	if hostingDirMode("/home/acme/public_html") != 0o750 {
		t.Fatal("public_html must not be world-readable")
	}
	if hostingDirMode("/home/acme/backups") != 0o750 {
		t.Fatal("private dirs stay 0750")
	}
}
