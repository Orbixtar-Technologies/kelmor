package operations

import "testing"

func TestHostingDirModeMakesPublicHTMLWorldReadable(t *testing.T) {
	if hostingDirMode("/home/acme/public_html") != 0o755 {
		t.Fatal("nginx must be able to traverse public_html")
	}
	if hostingDirMode("/home/acme/backups") != 0o750 {
		t.Fatal("private dirs stay 0750")
	}
}
