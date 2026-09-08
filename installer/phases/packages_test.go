package phases

import "testing"

func TestRejectUnknownPackage(t *testing.T) {
	if err := InstallPackages([]string{"nginx; rm -rf /"}); err == nil {
		t.Fatal("must reject")
	}
	if err := InstallPackages([]string{"totally-unknown-pkg"}); err == nil {
		t.Fatal("must reject")
	}
}
