package operations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const installerVsftpd = `listen=YES
listen_ipv6=NO
anonymous_enable=NO
local_enable=YES
write_enable=YES
chroot_local_user=YES
pam_service_name=vsftpd
guest_enable=YES
pasv_min_port=40000
pasv_max_port=40100
pasv_address=203.0.113.10
background=NO
`

func TestApplyFTPServerConfigWritesBannerAndPASV(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(h.Root, "etc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.Root, "etc/vsftpd.conf"), []byte(installerVsftpd), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := h.applyHostConfig(HostConfigSpec{
		FTPEnabled: true,
		PasvMin:    41000,
		PasvMax:    41100,
		FTPBanner:  "Kelmor FTP ready.",
	})
	if err != nil || !res.OK {
		t.Fatalf("apply ftp: %+v %v", res, err)
	}
	if !strings.Contains(res.Message, "ftp") {
		t.Fatalf("applied list should include ftp: %s", res.Message)
	}

	panel, err := os.ReadFile(filepath.Join(h.Root, "etc/panel/vsftpd-panel.conf"))
	if err != nil {
		t.Fatal(err)
	}
	overlay := string(panel)
	if !strings.Contains(overlay, "pasv_min_port=41000") || !strings.Contains(overlay, "pasv_max_port=41100") {
		t.Fatalf("panel overlay PASV: %s", overlay)
	}
	if !strings.Contains(overlay, "ftpd_banner=Kelmor FTP ready.") {
		t.Fatalf("panel overlay banner: %s", overlay)
	}
	if !strings.Contains(overlay, "listen=YES") {
		t.Fatalf("panel overlay listen: %s", overlay)
	}

	conf, err := os.ReadFile(filepath.Join(h.Root, "etc/vsftpd.conf"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(conf)
	if !strings.Contains(body, "pasv_min_port=41000") || !strings.Contains(body, "pasv_max_port=41100") {
		t.Fatalf("daemon PASV: %s", body)
	}
	if !strings.Contains(body, "ftpd_banner=Kelmor FTP ready.") {
		t.Fatalf("daemon banner: %s", body)
	}
	if !strings.Contains(body, "pasv_address=203.0.113.10") {
		t.Fatalf("installer pasv_address must stay: %s", body)
	}
	if !strings.Contains(body, "pam_service_name=vsftpd") {
		t.Fatalf("installer pam line must stay: %s", body)
	}
	if strings.Count(body, "pasv_min_port=") != 1 {
		t.Fatalf("PASV min must be upserted, not duplicated: %s", body)
	}
}

func TestApplyFTPServerConfigClearsBannerAndDisablesListen(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(h.Root, "etc"), 0o755); err != nil {
		t.Fatal(err)
	}
	seed := installerVsftpd + "ftpd_banner=old greeting\nbanner_file=/tmp/ftp.banner\n"
	if err := os.WriteFile(filepath.Join(h.Root, "etc/vsftpd.conf"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := h.applyHostConfig(HostConfigSpec{
		FTPEnabled: false,
		PasvMin:    40000,
		PasvMax:    40100,
		FTPBanner:  "",
	})
	if err != nil || !res.OK {
		t.Fatalf("apply ftp: %+v %v", res, err)
	}

	conf, err := os.ReadFile(filepath.Join(h.Root, "etc/vsftpd.conf"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(conf)
	if strings.Contains(body, "ftpd_banner=") {
		t.Fatalf("empty banner must drop ftpd_banner: %s", body)
	}
	if strings.Contains(body, "listen=YES") {
		t.Fatalf("disabled FTP must set listen=NO: %s", body)
	}
	if !strings.Contains(body, "listen=NO") {
		t.Fatalf("disabled FTP listen: %s", body)
	}
}

func TestApplyFTPServerConfigRejectsHostileBanner(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(h.Root, "etc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.Root, "etc/vsftpd.conf"), []byte(installerVsftpd), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := h.applyHostConfig(HostConfigSpec{
		FTPEnabled: true,
		PasvMin:    40000,
		PasvMax:    40100,
		FTPBanner:  "hello\nlisten=YES",
	})
	if err == nil || !strings.Contains(err.Error(), "banner") {
		t.Fatalf("multiline banner must be rejected: %v", err)
	}
}

func TestApplyFTPServerConfigWritesPanelOverlayWhenDaemonConfigMissing(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	res, err := h.applyHostConfig(HostConfigSpec{
		FTPEnabled: true,
		PasvMin:    40000,
		PasvMax:    40100,
	})
	if err != nil || !res.OK {
		t.Fatalf("PASV-only apply must not fail the rest of host config: %+v %v", res, err)
	}
	panel, err := os.ReadFile(filepath.Join(h.Root, "etc/panel/vsftpd-panel.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(panel), "pasv_min_port=40000") {
		t.Fatalf("sidecar: %s", panel)
	}
	if _, err := os.Stat(filepath.Join(h.Root, "etc/vsftpd.conf")); !os.IsNotExist(err) {
		t.Fatalf("must not invent vsftpd.conf: %v", err)
	}
}

func TestApplyFTPServerConfigFailsWhenDaemonConfigMissing(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	_, err := h.applyHostConfig(HostConfigSpec{
		FTPEnabled: true,
		PasvMin:    40000,
		PasvMax:    40100,
		FTPBanner:  "Kelmor FTP ready.",
	})
	if err == nil || !strings.Contains(err.Error(), "not present") {
		t.Fatalf("missing vsftpd.conf must fail honestly: %v", err)
	}
}
