package operations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const installerMaster = `smtp      inet  n       -       y       -       -       smtpd
# panel-submission
submission inet n       -       n       -       -       smtpd
  -o syslog_name=postfix/submission
  -o smtpd_tls_security_level=encrypt
  -o smtpd_sasl_auth_enable=yes
`

func TestApplyMailserverPortsWritesDovecotAndSubmission(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(h.Root, "etc/postfix"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.Root, "etc/postfix/master.cf"), []byte(installerMaster), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := h.applyHostConfig(HostConfigSpec{
		WriteMailserver: true,
		IMAPPort:        994,
		SubmissionPort:  2587,
	})
	if err != nil || !res.OK {
		t.Fatalf("apply mailserver: %+v %v", res, err)
	}
	if !strings.Contains(res.Message, "mailserver") {
		t.Fatalf("applied list should include mailserver: %s", res.Message)
	}

	panel, err := os.ReadFile(filepath.Join(h.Root, "etc/panel/mailserver-ports"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(panel), "imap=994") || !strings.Contains(string(panel), "submission=2587") {
		t.Fatalf("panel record: %s", panel)
	}

	dovecot, err := os.ReadFile(filepath.Join(h.Root, "etc/dovecot/conf.d/99-panel-ports.conf"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(dovecot)
	if !strings.Contains(body, "port = 994") || !strings.Contains(body, "inet_listener imaps") {
		t.Fatalf("dovecot ports: %s", body)
	}
	if !strings.Contains(body, "inet_listener imap") || !strings.Contains(body, "port = 0") {
		t.Fatalf("plaintext IMAP must stay disabled: %s", body)
	}

	master, err := os.ReadFile(filepath.Join(h.Root, "etc/postfix/master.cf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(master), "2587") || !strings.Contains(string(master), "panel-submission") {
		t.Fatalf("master.cf submission: %s", master)
	}
	if strings.Contains(string(master), "\nsubmission inet") {
		t.Fatalf("named submission service must be rewritten to the port: %s", master)
	}
	if !strings.Contains(string(master), "smtp      inet") {
		t.Fatalf("smtp listener must stay: %s", master)
	}
}

func TestApplyMailserverPortsRejectsCollidingPorts(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	_, err := h.applyHostConfig(HostConfigSpec{
		WriteMailserver: true,
		IMAPPort:        587,
		SubmissionPort:  587,
	})
	if err == nil {
		t.Fatal("identical mail ports must fail")
	}
}

func TestApplyMailserverDefaultPortsWithoutMasterCF(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	res, err := h.applyHostConfig(HostConfigSpec{WriteMailserver: true})
	if err != nil || !res.OK {
		t.Fatalf("default mail ports: %+v %v", res, err)
	}
	dovecot, err := os.ReadFile(filepath.Join(h.Root, "etc/dovecot/conf.d/99-panel-ports.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dovecot), "port = 993") {
		t.Fatalf("default IMAP: %s", dovecot)
	}
}

func TestApplyMailserverCustomPortsRequireMasterCF(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	_, err := h.applyHostConfig(HostConfigSpec{
		WriteMailserver: true,
		IMAPPort:        994,
		SubmissionPort:  2587,
	})
	if err == nil {
		t.Fatal("custom submission without master.cf must fail")
	}
}
