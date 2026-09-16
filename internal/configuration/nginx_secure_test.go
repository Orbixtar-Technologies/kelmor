package configuration

import "testing"

func TestNginxSiteRejectsForeignDocumentRoot(t *testing.T) {
	_, err := NginxSiteChecked(WebsiteSpec{
		WebsiteID:    "abc",
		Account:      "acme42",
		Domain:       "acme.test",
		DocumentRoot: "/home/other/public_html",
		Runtime:      "php",
		Enabled:      true,
	})
	if err == nil {
		t.Fatal("expected foreign document root to fail")
	}
}

func TestNginxSiteRejectsDirectiveInjection(t *testing.T) {
	_, err := NginxSiteChecked(WebsiteSpec{
		WebsiteID:    "abc",
		Account:      "acme42",
		Domain:       "acme.test",
		DocumentRoot: "/home/acme42/public_html; return 200; #",
		Runtime:      "php",
		Enabled:      true,
	})
	if err == nil {
		t.Fatal("expected nginx injection to fail")
	}
}
