package operations

import (
	"fmt"
	"strconv"
	"strings"
)

func mailPortOK(n int) bool {
	return n >= 1 && n <= 65535
}

func (h *Host) applyMailserverPorts(spec HostConfigSpec) error {
	imap := spec.IMAPPort
	if imap == 0 {
		imap = 993
	}
	submission := spec.SubmissionPort
	if submission == 0 {
		submission = 587
	}
	if !mailPortOK(imap) || !mailPortOK(submission) {
		return fmt.Errorf("mail ports are invalid")
	}
	if imap == submission {
		return fmt.Errorf("IMAP and submission ports must differ")
	}
	record := fmt.Sprintf("imap=%d\nsubmission=%d\n", imap, submission)
	if _, err := h.ApplyFile("/etc/panel/mailserver-ports", []byte(record), 0o644); err != nil {
		return err
	}
	dovecot := fmt.Sprintf(`# Kelmor mail ports — written by ApplyHostConfig
service imap-login {
  inet_listener imap {
    port = 0
  }
  inet_listener imaps {
    port = %d
    ssl = yes
  }
}
`, imap)
	if _, err := h.ApplyFile("/etc/dovecot/conf.d/99-panel-ports.conf", []byte(dovecot), 0o644); err != nil {
		return err
	}
	existing, err := h.readManaged("/etc/postfix/master.cf", 1<<20)
	if err != nil {
		if submission != 587 {
			return fmt.Errorf("Postfix master.cf is not present")
		}
	} else {
		rewritten := rewriteSubmissionPort(string(existing), submission)
		if _, err := h.ApplyFile("/etc/postfix/master.cf", []byte(rewritten), 0o644); err != nil {
			return err
		}
	}
	if h.live() {
		_ = reloadNamedService("dovecot")
		_ = reloadNamedService("postfix")
	}
	return nil
}

func rewriteSubmissionPort(existing string, port int) string {
	service := strconv.Itoa(port)
	lines := strings.Split(existing, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "# panel-submission" {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			trimmed := strings.TrimSpace(lines[j])
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			fields := strings.Fields(trimmed)
			if len(fields) < 2 || fields[1] != "inet" {
				break
			}
			idx := strings.Index(lines[j], fields[0])
			if idx < 0 {
				break
			}
			lines[j] = lines[j][:idx] + service + lines[j][idx+len(fields[0]):]
			return strings.Join(lines, "\n")
		}
		break
	}
	out := existing
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out + panelSubmissionService(service)
}

func panelSubmissionService(port string) string {
	return `# panel-submission
` + port + ` inet n       -       n       -       -       smtpd
  -o syslog_name=postfix/submission
  -o smtpd_tls_security_level=encrypt
  -o smtpd_sasl_auth_enable=yes
  -o smtpd_sasl_type=dovecot
  -o smtpd_sasl_path=private/auth
  -o smtpd_client_restrictions=permit_sasl_authenticated,reject
  -o smtpd_recipient_restrictions=permit_sasl_authenticated,reject
  -o smtpd_relay_restrictions=permit_sasl_authenticated,reject
  -o smtpd_sender_restrictions=reject_sender_login_mismatch
  -o milter_macro_daemon_name=ORIGINATING
`
}
