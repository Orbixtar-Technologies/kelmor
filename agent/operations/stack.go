package operations

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/hosting-panel/panel/internal/pkg/validate"
	paneltls "github.com/hosting-panel/panel/internal/tls"
)

func (h *Host) applyMailMaps(virtual, domains, passwd string) (Result, error) {
	if _, err := h.ApplyFile("/var/lib/panel/mail/virtual", []byte(virtual), 0o640); err != nil {
		return Result{}, err
	}
	if _, err := h.ApplyFile("/var/lib/panel/mail/vdomains", []byte(domains), 0o640); err != nil {
		return Result{}, err
	}
	if _, err := h.ApplyFile("/var/lib/panel/mail/passwd", []byte(passwd), 0o640); err != nil {
		return Result{}, err
	}
	return Result{OK: true, Message: "mail maps written", ObservedState: "applied"}, nil
}

func (h *Host) createMailboxHome(domain, local string, uid, gid int) (Result, error) {
	if _, err := validate.NormalizeDomain(domain); err != nil {
		return Result{}, err
	}
	if err := validate.LocalPart(local); err != nil {
		return Result{}, err
	}
	base := "/var/vmail/" + domain + "/" + local
	for _, d := range []string{"", "Maildir", "Maildir/new", "Maildir/cur", "Maildir/tmp"} {
		p := base
		if d != "" {
			p = filepath.Join(base, d)
		}
		if _, err := h.CreateDirectoryTree(p, 0o750); err != nil {
			return Result{}, err
		}
	}
	meta := fmt.Sprintf("uid=%d gid=%d created=%s\n", uid, gid, time.Now().UTC().Format(time.RFC3339))
	if _, err := h.ApplyFile(base+"/.panel-mailbox", []byte(meta), 0o640); err != nil {
		return Result{}, err
	}
	return Result{OK: true, ObservedState: "exists"}, nil
}

func (h *Host) applyDNSZone(name, body string) (Result, error) {
	if _, err := validate.NormalizeDomain(name); err != nil {
		return Result{}, err
	}
	path := "/var/lib/panel/dns/zones/" + name + ".zone"
	if _, err := h.ApplyFile(path, []byte(body), 0o644); err != nil {
		return Result{}, err
	}
	return Result{OK: true, Message: "zone written", ObservedState: "applied"}, nil
}

func (h *Host) applyAccountCron(username, body string) (Result, error) {
	if err := validate.Username(username); err != nil {
		return Result{}, err
	}
	path := "/var/lib/panel/cron/" + username
	if _, err := h.ApplyFile(path, []byte(body), 0o600); err != nil {
		return Result{}, err
	}
	return Result{OK: true, ObservedState: "applied"}, nil
}

func (h *Host) applyAuthorizedKeys(username, body string) (Result, error) {
	if err := validate.Username(username); err != nil {
		return Result{}, err
	}
	path := "/home/" + username + "/.ssh/authorized_keys"
	if _, err := h.ApplyFile(path, []byte(body), 0o600); err != nil {
		return Result{}, err
	}
	return Result{OK: true, ObservedState: "applied"}, nil
}

func (h *Host) issueDevCertificate(hostname string, days int) (Result, error) {
	if _, err := validate.NormalizeDomain(hostname); err != nil {
		return Result{}, err
	}
	if days <= 0 {
		days = 90
	}
	cert, key, err := paneltls.SelfSigned(hostname, time.Now().AddDate(0, 0, days))
	if err != nil {
		return Result{}, err
	}
	base := "/var/lib/panel/certs/" + hostname
	if _, err := h.ApplyFile(base+".crt", cert, 0o644); err != nil {
		return Result{}, err
	}
	if _, err := h.ApplyFile(base+".key", key, 0o600); err != nil {
		return Result{}, err
	}
	return Result{OK: true, Message: "certificate written", ObservedState: "active"}, nil
}

func decodeMaps(raw json.RawMessage) (virtual, domains, passwd string, err error) {
	var p struct {
		Virtual string `json:"virtual"`
		Domains string `json:"domains"`
		Passwd  string `json:"passwd"`
	}
	if err = json.Unmarshal(raw, &p); err != nil {
		return "", "", "", err
	}
	if strings.ContainsAny(p.Virtual, "\x00") {
		return "", "", "", fmt.Errorf("NUL in mail map")
	}
	return p.Virtual, p.Domains, p.Passwd, nil
}
