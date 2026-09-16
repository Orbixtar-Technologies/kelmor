package configuration

import (
	"strings"
	"testing"
)

func TestPortalHTTPRedirect(t *testing.T) {
	conf := PortalHTTPRedirect("lab.kelmor.host", 2087)
	if err := ValidateNginx(conf); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"server_name lab.kelmor.host www.lab.kelmor.host;",
		"server_name kelmor.host www.kelmor.host;",
		"return 301 https://lab.kelmor.host:2087$request_uri;",
		"/.well-known/acme-challenge/",
	} {
		if !contains(conf, want) {
			t.Fatalf("missing %q in %s", want, conf)
		}
	}
	if PortalHTTPRedirect("localhost", 2087) != "" {
		t.Fatal("localhost must not emit portal HTTP config")
	}
}

func TestNginxAliasServerNames(t *testing.T) {
	conf := NginxSite(WebsiteSpec{
		WebsiteID: "abc", Domain: "acme.test", DocumentRoot: "/home/acme/public_html",
		Runtime: "php", Enabled: true, Aliases: []string{"www.acme.test", "parked.test"},
	})
	if err := ValidateNginx(conf); err != nil {
		t.Fatal(err)
	}
	if !contains(conf, "server_name acme.test www.acme.test parked.test;") {
		t.Fatal(conf)
	}
}

func TestNginxSuspendedReturns503(t *testing.T) {
	conf := NginxSite(WebsiteSpec{WebsiteID: "abc", Domain: "acme.test", DocumentRoot: "/home/acme/public_html", Runtime: "php", Enabled: false})
	if err := ValidateNginx(conf); err != nil {
		t.Fatal(err)
	}
	if !contains(conf, "return 503") || contains(conf, "fastcgi_pass") {
		t.Fatal(conf)
	}
}

func TestNginxConcurrentLimitConn(t *testing.T) {
	conf := NginxSite(WebsiteSpec{
		WebsiteID: "abc", Account: "acme42", Domain: "acme.test",
		DocumentRoot: "/home/acme/public_html", Runtime: "php", Enabled: true,
		ConcurrentWebRequests: 12,
	})
	if err := ValidateNginx(conf); err != nil {
		t.Fatal(err)
	}
	if !contains(conf, "limit_conn panel_acct 12") || !contains(conf, `set $panel_account "acme42"`) {
		t.Fatal(conf)
	}
	zone := NginxConnZone(map[string]string{"acme.test": "acme42"})
	if !contains(zone, "limit_conn_zone") || !contains(zone, "acme.test acme42") {
		t.Fatal(zone)
	}
}

func TestNginxBandwidthHoldReturns509(t *testing.T) {
	conf := NginxSite(WebsiteSpec{WebsiteID: "abc", Domain: "acme.test", DocumentRoot: "/home/acme/public_html", Runtime: "php", Enabled: true, BandwidthHold: true})
	if err := ValidateNginx(conf); err != nil {
		t.Fatal(err)
	}
	if !contains(conf, "return 509") || contains(conf, "fastcgi_pass") {
		t.Fatal(conf)
	}
	if !contains(conf, "acme-challenge") {
		t.Fatal("held vhost must keep HTTP-01")
	}
}

func TestNginxHTTPSRedirectKeepsHTTP01(t *testing.T) {
	conf := NginxSite(WebsiteSpec{
		WebsiteID: "abc", Domain: "acme.test", DocumentRoot: "/home/acme/public_html",
		Runtime: "php", HTTPSRedirect: true, Enabled: true,
		TLSCert: "/var/lib/panel/certs/acme.test.crt",
		TLSKey:  "/var/lib/panel/certs/acme.test.key",
	})
	if err := ValidateNginx(conf); err != nil {
		t.Fatal(err)
	}
	if !contains(conf, "location / { return 301 https://$host$request_uri; }") {
		t.Fatal(conf)
	}
	if contains(conf, "if ($scheme = http)") {
		t.Fatal("HTTP-01 must not sit behind a server-level HTTPS if")
	}
	if !contains(conf, "/.well-known/acme-challenge/") {
		t.Fatal("redirecting vhost must keep HTTP-01")
	}
	if !contains(conf, "root "+ACMEHTTP01Root) {
		t.Fatal("HTTP-01 must use the AppArmor-reachable webroot")
	}
	if contains(conf, "root /var/lib/panel/acme-www") {
		t.Fatal("HTTP-01 must not use /var/lib/panel/acme-www")
	}
}

func TestNginxToolSiteRedirectsHTTPWhenTLS(t *testing.T) {
	conf := NginxToolSite(ToolSiteSpec{
		SiteID: "webmail-acme.test", Hostname: "webmail.acme.test",
		DocumentRoot: "/usr/share/roundcube",
		TLSCert:      "/var/lib/panel/certs/acme.test.crt",
		TLSKey:       "/var/lib/panel/certs/acme.test.key",
	})
	if err := ValidateNginx(conf); err != nil {
		t.Fatal(err)
	}
	if !contains(conf, "server_name webmail.acme.test;") {
		t.Fatal(conf)
	}
	if !contains(conf, "location / { return 301 https://$host$request_uri; }") {
		t.Fatal(conf)
	}
	if !contains(conf, "root /usr/share/roundcube") {
		t.Fatal(conf)
	}
}

func TestAccountServiceHostnames(t *testing.T) {
	got := strings.Join(AccountServiceHostnames(), ",")
	for _, name := range []string{"www", "mail", "ftp", "webmail", "phpmyadmin"} {
		if !contains(got, name) {
			t.Fatalf("missing %s in %s", name, got)
		}
	}
}

func TestNginxACMEDefaultServerUsesWWWRoot(t *testing.T) {
	conf := NginxACMEDefaultServer()
	if err := ValidateNginx(conf); err != nil {
		t.Fatal(err)
	}
	if !contains(conf, "listen 80 default_server") {
		t.Fatal(conf)
	}
	if !contains(conf, "root "+ACMEHTTP01Root) {
		t.Fatal(conf)
	}
	if contains(conf, "root /var/lib/panel/acme-www") {
		t.Fatal("default HTTP-01 vhost must not use acme-www")
	}
}

func TestNginxSiteValid(t *testing.T) {
	conf := NginxSite(WebsiteSpec{WebsiteID: "abc", Domain: "acme.test", DocumentRoot: "/home/acme/public_html", Runtime: "php", PHPVersion: "8.3", HTTPSRedirect: true, Revision: 3, Enabled: true})
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

func TestNginxPythonProxiesOnHTTPS(t *testing.T) {
	conf := NginxSite(WebsiteSpec{
		WebsiteID: "py1", Domain: "python.acme.test", DocumentRoot: "/home/acme/python",
		Runtime: "python", TLSCert: "/var/lib/panel/certs/python.acme.test.crt",
		TLSKey: "/var/lib/panel/certs/python.acme.test.key", Enabled: true,
	})
	if err := ValidateNginx(conf); err != nil {
		t.Fatal(err)
	}
	if !contains(conf, "unix:/run/panel/apps/py1.sock") {
		t.Fatal(conf)
	}
}

func TestSystemdSliceUsesPackageIO(t *testing.T) {
	s := SystemdSlice("acme42", 80, 64<<20, 40, 250, 500)
	if !contains(s, "IOWeight=250") || !contains(s, "TasksMax=40") || !contains(s, "panel_iops=500") {
		t.Fatal(s)
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
