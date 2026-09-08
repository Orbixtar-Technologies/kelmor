package operations

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseNginxBodyBytesCombinedAndPanel(t *testing.T) {
	when, n, ok := parseNginxBodyBytes(`127.0.0.1 - - [08/Sep/2026:19:00:01 +0000] "GET / HTTP/1.1" 200 4096 "-" "curl"`)
	if !ok || n != 4096 || when.Month() != time.September {
		t.Fatalf("combined: ok=%v n=%d when=%v", ok, n, when)
	}
	when, n, ok = parseNginxBodyBytes(`2026-09-08T19:00:01Z 128`)
	if !ok || n != 128 || when.Year() != 2026 {
		t.Fatalf("panel_bw: ok=%v n=%d when=%v", ok, n, when)
	}
	if _, _, ok := parseNginxBodyBytes(""); ok {
		t.Fatal("empty line")
	}
}

func TestSumNginxBandwidthFiltersMonth(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	id := "sitebw1"
	if err := os.MkdirAll(filepath.Join(root, "var/log/nginx"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "etc/nginx/panel-sites"), 0o755); err != nil {
		t.Fatal(err)
	}
	conf := "server {\n    root /home/bwuser/public_html;\n    server_name bw.test;\n}\n"
	if err := os.WriteFile(filepath.Join(root, "etc/nginx/panel-sites", id+".conf"), []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	prev := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	body := "127.0.0.1 - - [" + now.Format("02/Jan/2006:15:04:05 -0700") + "] \"GET / HTTP/1.1\" 200 100 \"-\" \"c\"\n" +
		"127.0.0.1 - - [" + prev.Format("02/Jan/2006:15:04:05 -0700") + "] \"GET / HTTP/1.1\" 200 9999 \"-\" \"c\"\n" +
		now.Format(time.RFC3339) + " 50\n"
	if err := os.WriteFile(filepath.Join(root, "var/log/nginx", id+".access.log"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ids := h.websiteIDsForAccount("bwuser")
	if len(ids) != 1 || ids[0] != id {
		t.Fatalf("ids=%v", ids)
	}
	got := h.sumNginxBandwidth(ids, now)
	if got != 150 {
		t.Fatalf("sum=%d", got)
	}
}

func TestApplyWebsiteBandwidthHold509(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	if _, err := h.applyWebsite("hold1", "acme42", "hold.test", "/home/acme42/public_html", "php", "", "", false, true, true, 0); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "etc/nginx/panel-sites/hold1.conf"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !containsStr(s, "return 509") || containsStr(s, "fastcgi_pass") {
		t.Fatal(s)
	}
	if !containsStr(s, "acme-challenge") {
		t.Fatal("ACME location required while held")
	}
}

func TestApplyWebsiteWritesConnLimit(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	if _, err := h.applyWebsite("c1", "acme42", "c.test", "/home/acme42/public_html", "php", "", "", false, true, false, 7); err != nil {
		t.Fatal(err)
	}
	zone, err := os.ReadFile(filepath.Join(root, "etc/nginx/conf.d/panel-conn-limit.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !containsStr(string(zone), "limit_conn_zone") || !containsStr(string(zone), "c.test acme42") {
		t.Fatal(string(zone))
	}
	body, err := os.ReadFile(filepath.Join(root, "etc/nginx/panel-sites/c1.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !containsStr(string(body), "limit_conn panel_acct 7") {
		t.Fatal(string(body))
	}
}

func TestApplyWebsiteSuspendWinsOverHold(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	if _, err := h.applyWebsite("sus1", "acme42", "sus.test", "/home/acme42/public_html", "php", "", "", false, false, true, 0); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "etc/nginx/panel-sites/sus1.conf"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !containsStr(s, "return 503") || containsStr(s, "return 509") {
		t.Fatal(s)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(sub) > 0 && indexOf(s, sub) >= 0))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
