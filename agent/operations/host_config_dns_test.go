package operations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const installerPdns = `setuid=pdns
setgid=pdns
launch=
bind-config=/etc/powerdns/named.conf
local-address=127.0.0.1
local-port=53
include-dir=/etc/powerdns/pdns.d
`

func TestApplyNameserverListenWritesPdnsLocalAddress(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(h.Root, "etc/powerdns"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.Root, "etc/powerdns/pdns.conf"), []byte(installerPdns), 0o640); err != nil {
		t.Fatal(err)
	}

	res, err := h.applyHostConfig(HostConfigSpec{
		WriteNameserver:    true,
		NameserverSoftware: "pdns",
		NameserverListen:   "127.0.0.1, 203.0.113.10",
	})
	if err != nil || !res.OK {
		t.Fatalf("apply nameserver: %+v %v", res, err)
	}
	if !strings.Contains(res.Message, "nameserver") {
		t.Fatalf("applied list should include nameserver: %s", res.Message)
	}

	panel, err := os.ReadFile(filepath.Join(h.Root, "etc/panel/nameserver-selection"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(panel), "software=pdns") || !strings.Contains(string(panel), "listen_address=127.0.0.1,203.0.113.10") {
		t.Fatalf("panel record: %s", panel)
	}

	dropIn, err := os.ReadFile(filepath.Join(h.Root, "etc/powerdns/pdns.d/99-panel-listen.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dropIn), "local-address=127.0.0.1,203.0.113.10") {
		t.Fatalf("drop-in: %s", dropIn)
	}

	conf, err := os.ReadFile(filepath.Join(h.Root, "etc/powerdns/pdns.conf"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(conf)
	if !strings.Contains(body, "local-address=127.0.0.1,203.0.113.10") {
		t.Fatalf("pdns.conf listen: %s", body)
	}
	if strings.Count(body, "local-address=") != 1 {
		t.Fatalf("local-address must be upserted: %s", body)
	}
	if !strings.Contains(body, "bind-config=/etc/powerdns/named.conf") {
		t.Fatalf("installer bind-config must stay: %s", body)
	}
}

func TestApplyNameserverListenRejectsInvalidAddress(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	_, err := h.applyHostConfig(HostConfigSpec{
		WriteNameserver:    true,
		NameserverSoftware: "pdns",
		NameserverListen:   "not-an-ip; rm -rf /",
	})
	if err == nil {
		t.Fatal("invalid listen must fail")
	}
}

func TestApplyNameserverDisabledDoesNotRequirePdnsConf(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	res, err := h.applyHostConfig(HostConfigSpec{
		WriteNameserver:    true,
		NameserverSoftware: "disabled",
	})
	if err != nil || !res.OK {
		t.Fatalf("disable nameserver: %+v %v", res, err)
	}
	panel, err := os.ReadFile(filepath.Join(h.Root, "etc/panel/nameserver-selection"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(panel), "software=disabled") {
		t.Fatalf("panel record: %s", panel)
	}
}
