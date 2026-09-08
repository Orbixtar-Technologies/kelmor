package operations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDSRecords(t *testing.T) {
	body := `livehost.test. IN DS 12345 13 2 ABCDEF
; comment
livehost.test. 3600 IN DS 99 8 2 deadbeef
`
	recs := ParseDSRecords("livehost.test", body)
	if len(recs) != 2 {
		t.Fatalf("%v", recs)
	}
	if recs[0].Content != "12345 13 2 ABCDEF" {
		t.Fatalf("%q", recs[0].Content)
	}
	if recs[1].TTL != 3600 || !strings.Contains(recs[1].Content, "deadbeef") {
		t.Fatalf("%+v", recs[1])
	}
}

func TestSetDNSSECSandboxWritesDS(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	st, err := h.setDNSSEC("acme.test", true)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Enabled || len(st.DS) == 0 {
		t.Fatalf("%+v", st)
	}
	got, err := h.getDSRecords("acme.test")
	if err != nil || len(got.DS) == 0 {
		t.Fatalf("%+v %v", got, err)
	}
	path := filepath.Join(h.Root, "var/lib/panel/dns/ds/acme.test.ds")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	off, err := h.setDNSSEC("acme.test", false)
	if err != nil || off.Enabled {
		t.Fatalf("%+v %v", off, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("ds file remains")
	}
}
