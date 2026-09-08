package configuration

import "testing"

func TestNginxSiteValid(t *testing.T) {
	conf := NginxSite(WebsiteSpec{WebsiteID: "abc", Domain: "acme.test", DocumentRoot: "/home/acme/public_html", Runtime: "php", PHPVersion: "8.3", HTTPSRedirect: true, Revision: 3})
	if err := ValidateNginx(conf); err != nil {
		t.Fatal(err)
	}
	if !contains(conf, "SCRIPT_FILENAME") {
		t.Fatal("php location must set SCRIPT_FILENAME")
	}
	if err := ValidateNginx("not a server"); err == nil {
		t.Fatal("expected invalid")
	}
}

func TestPHPPool(t *testing.T) {
	p := PHPPool("acme42", "8.5", 8)
	if p == "" || !contains(p, "open_basedir") {
		t.Fatal(p)
	}
	alt := PHPPoolFor("migrated", "panel-sftp", "8.3", 4)
	if !contains(alt, "group = panel-sftp") {
		t.Fatal(alt)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})())
}
