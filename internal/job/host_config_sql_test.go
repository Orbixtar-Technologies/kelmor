package job

import (
	"testing"

	"github.com/hosting-panel/panel/internal/hostconfig"
)

func TestHostConfigSpecMapsPostgresListenAndAuth(t *testing.T) {
	spec := hostConfigSpec(hostconfig.File{Values: map[string]map[string]string{
		"postgres": {"listen": "0.0.0.0", "auth": "md5"},
	}})
	if !spec.WritePostgres {
		t.Fatal("postgres settings must mark WritePostgres")
	}
	if spec.PostgresListen != "0.0.0.0" || spec.PostgresAuth != "md5" {
		t.Fatalf("postgres spec: %+v", spec)
	}
}

func TestHostConfigSpecMapsFTPBannerAndPASV(t *testing.T) {
	spec := hostConfigSpec(hostconfig.File{Values: map[string]map[string]string{
		"ftp_server":    {"pasv_min": "41000", "pasv_max": "41100", "banner": "Kelmor FTP ready."},
		"ftp_selection": {"daemon": "vsftpd"},
	}})
	if spec.PasvMin != 41000 || spec.PasvMax != 41100 {
		t.Fatalf("pasv spec: %+v", spec)
	}
	if spec.FTPBanner != "Kelmor FTP ready." {
		t.Fatalf("banner spec: %+v", spec)
	}
	if !spec.FTPEnabled {
		t.Fatal("vsftpd selection must keep FTP enabled")
	}

	disabled := hostConfigSpec(hostconfig.File{Values: map[string]map[string]string{
		"ftp_selection": {"daemon": "disabled"},
		"ftp_server":    {"banner": "still stored"},
	}})
	if disabled.FTPEnabled {
		t.Fatal("disabled daemon must turn FTP off")
	}
	if disabled.FTPBanner != "still stored" {
		t.Fatalf("banner must still map when FTP is disabled: %+v", disabled)
	}
}

func TestHostConfigSpecMapsNameserverListen(t *testing.T) {
	spec := hostConfigSpec(hostconfig.File{Values: map[string]map[string]string{
		"nameserver_selection": {"software": "pdns", "listen_address": "127.0.0.1,203.0.113.10"},
	}})
	if !spec.WriteNameserver {
		t.Fatal("nameserver settings must mark WriteNameserver")
	}
	if spec.NameserverSoftware != "pdns" {
		t.Fatalf("software %q", spec.NameserverSoftware)
	}
	if spec.NameserverListen != "127.0.0.1,203.0.113.10" {
		t.Fatalf("listen %q", spec.NameserverListen)
	}

	disabled := hostConfigSpec(hostconfig.File{Values: map[string]map[string]string{
		"nameserver_selection": {"software": "disabled"},
	}})
	if !disabled.WriteNameserver || disabled.NameserverSoftware != "disabled" {
		t.Fatalf("disabled nameserver spec: %+v", disabled)
	}
}

func TestHostConfigSpecMapsMailserverPorts(t *testing.T) {
	spec := hostConfigSpec(hostconfig.File{Values: map[string]map[string]string{
		"mailserver": {"imap": "994", "submission": "2587"},
	}})
	if !spec.WriteMailserver {
		t.Fatal("mailserver settings must mark WriteMailserver")
	}
	if spec.IMAPPort != 994 || spec.SubmissionPort != 2587 {
		t.Fatalf("mail ports: %+v", spec)
	}
}
