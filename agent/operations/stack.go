package operations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hosting-panel/panel/internal/pkg/validate"
	paneltls "github.com/hosting-panel/panel/internal/tls"
)

func (h *Host) applyMailMaps(virtual, domains, passwd, uids, gids string) (Result, error) {
	if _, err := h.ApplyFile("/var/lib/panel/mail/virtual", []byte(virtual), 0o640); err != nil {
		return Result{}, err
	}
	if _, err := h.ApplyFile("/var/lib/panel/mail/vdomains", []byte(domains), 0o640); err != nil {
		return Result{}, err
	}
	if _, err := h.ApplyFile("/var/lib/panel/mail/passwd", []byte(passwd), 0o640); err != nil {
		return Result{}, err
	}
	if uids != "" {
		if _, err := h.ApplyFile("/var/lib/panel/mail/uids", []byte(uids), 0o640); err != nil {
			return Result{}, err
		}
	}
	if gids != "" {
		if _, err := h.ApplyFile("/var/lib/panel/mail/gids", []byte(gids), 0o640); err != nil {
			return Result{}, err
		}
	}
	if h.live() {
		_ = os.MkdirAll("/var/lib/panel/mail", 0o755)
		_ = os.Chmod("/var/lib/panel/mail", 0o755)
		for _, mapfile := range []string{"/var/lib/panel/mail/virtual", "/var/lib/panel/mail/vdomains", "/var/lib/panel/mail/uids", "/var/lib/panel/mail/gids"} {
			if _, err := os.Stat(mapfile); err != nil {
				continue
			}
			_ = os.Chmod(mapfile, 0o644)
			if out, err := runFixed("/usr/sbin/postmap", mapfile); err != nil {
				return Result{}, fmt.Errorf("postmap: %s", strings.TrimSpace(string(out)))
			}
			_ = os.Chmod(mapfile+".db", 0o644)
		}
		if p, err := h.resolve("/var/lib/panel/mail/passwd"); err == nil {
			_ = os.Chmod(p, 0o644)
		}
		_, _ = runFixed("/usr/sbin/postfix", "reload")
		_, _ = runFixed("/usr/bin/doveadm", "reload")
	}
	return Result{OK: true, Message: "mail maps written", ObservedState: "applied"}, nil
}

func (h *Host) applyACMEChallenge(token, body string) (Result, error) {
	if token == "" || strings.ContainsAny(token, "/\\") {
		return Result{}, fmt.Errorf("invalid ACME token")
	}
	path := "/var/lib/panel/acme-www/.well-known/acme-challenge/" + token
	if _, err := h.ApplyFile(path, []byte(body), 0o644); err != nil {
		return Result{}, err
	}
	return Result{OK: true, ObservedState: "published"}, nil
}

func (h *Host) applyAppUnit(websiteID, account, runtime, workDir, command string) (Result, error) {
	if err := validate.Username(account); err != nil {
		return Result{}, err
	}
	if command == "" {
		switch runtime {
		case "node":
			command = "/usr/bin/node server.js"
		case "python":
			command = "/usr/bin/python3 -m http.server 8080"
		default:
			return Result{OK: true, Message: "no unit for runtime"}, nil
		}
	}
	if workDir == "" {
		workDir = "/home/" + account + "/apps/" + websiteID
	}
	if _, err := h.CreateDirectoryTree(workDir, 0o750); err != nil {
		return Result{}, err
	}
	if _, err := h.CreateDirectoryTree("/run/panel/apps", 0o1777); err != nil {
		return Result{}, err
	}
	if h.live() {
		_ = os.Chmod("/run/panel/apps", 0o1777)
	}
	sock := "/run/panel/apps/" + websiteID + ".sock"
	switch runtime {
	case "node":
		stub := fmt.Sprintf("const http=require('http');\nconst fs=require('fs');\nconst sock=%q;\ntry{fs.unlinkSync(sock)}catch(e){}\nconst s=http.createServer((q,r)=>{r.writeHead(200,{'content-type':'text/plain'});r.end('node %s\\n');});\ns.listen(sock,()=>{try{fs.chmodSync(sock,0o666)}catch(e){}});\n", sock, websiteID)
		_, _ = h.ApplyFile(workDir+"/server.js", []byte(stub), 0o644)
		command = "/usr/bin/node server.js"
	case "python":
		stub := fmt.Sprintf("import os, socket\nfrom http.server import BaseHTTPRequestHandler, ThreadingHTTPServer\nclass S(ThreadingHTTPServer):\n    address_family = socket.AF_UNIX\nclass H(BaseHTTPRequestHandler):\n    def address_string(self):\n        return 'unix'\n    def log_message(self, fmt, *args):\n        pass\n    def do_GET(self):\n        self.send_response(200); self.end_headers(); self.wfile.write(b'python %s\\n')\nsock = %q\ntry: os.unlink(sock)\nexcept FileNotFoundError: pass\nhttpd = S(sock, H)\nos.chmod(sock, 0o666)\nhttpd.serve_forever()\n", websiteID, sock)
		_, _ = h.ApplyFile(workDir+"/app.py", []byte(stub), 0o644)
		command = "/usr/bin/python3 app.py"
	}
	body := fmt.Sprintf("[Unit]\nDescription=panel app %s\n[Service]\nUser=%s\nWorkingDirectory=%s\nExecStart=%s\nRestart=on-failure\nSlice=panel-account-%s.slice\n[Install]\nWantedBy=multi-user.target\n",
		websiteID, account, workDir, command, account)
	path := "/etc/systemd/system/panel-app-" + websiteID + ".service"
	if _, err := h.ApplyFile(path, []byte(body), 0o644); err != nil {
		return Result{}, err
	}
	if h.live() {
		_, _ = runFixed("/bin/systemctl", "daemon-reload")
		_, _ = runFixed("/bin/systemctl", "start", "panel-app-"+websiteID+".service")
		if _, err := os.Stat(sock); err != nil {
			_ = startAccountProcess(account, workDir, runtime)
		}
	}
	return Result{OK: true, ObservedState: "applied"}, nil
}

func startAccountProcess(account, workDir, runtime string) error {
	switch runtime {
	case "node":
		bin := "/usr/bin/node"
		if _, err := os.Stat(bin); err != nil {
			if _, err := os.Stat("/exec-daemon/node"); err == nil {
				bin = "/exec-daemon/node"
			} else if _, err := os.Stat("/usr/bin/nodejs"); err == nil {
				bin = "/usr/bin/nodejs"
			}
		}
		return startDetached("/usr/sbin/runuser", workDir, "-u", account, "--", bin, "server.js")
	case "python":
		return startDetached("/usr/sbin/runuser", workDir, "-u", account, "--", "/usr/bin/python3", "app.py")
	default:
		return nil
	}
}

func (h *Host) createMailboxHome(domain, local string, uid, gid int) (Result, error) {
	if _, err := validate.NormalizeDomain(domain); err != nil {
		return Result{}, err
	}
	if err := validate.LocalPart(local); err != nil {
		return Result{}, err
	}
	if h.live() {
		_ = os.MkdirAll("/var/vmail", 0o755)
		_ = os.Chmod("/var/vmail", 0o755)
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
	if h.live() && uid >= 20000 {
		if domainDir, err := h.resolve("/var/vmail/" + domain); err == nil {
			_ = os.Chmod(domainDir, 0o755)
		}
		real, err := h.resolve(base)
		if err == nil {
			_ = os.Chown(real, uid, gid)
			_ = filepath.Walk(real, func(p string, info os.FileInfo, err error) error {
				if err == nil {
					_ = os.Chown(p, uid, gid)
				}
				return nil
			})
		}
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
	inc := fmt.Sprintf("zone \"%s\" { type master; file \"%s.zone\"; };\n", name, name)
	named := "/var/lib/panel/dns/named-zones.conf"
	prev := ""
	if rp, err := h.resolve(named); err == nil {
		if b, err := os.ReadFile(rp); err == nil {
			prev = string(b)
		}
	}
	if !strings.Contains(prev, `zone "`+name+`"`) {
		if _, err := h.ApplyFile(named, []byte(prev+inc), 0o644); err != nil {
			return Result{}, err
		}
	}
	if h.live() {
		_, _ = runFixed("/usr/bin/pdns_control", "rediscover")
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

func decodeMaps(raw json.RawMessage) (virtual, domains, passwd, uids, gids string, err error) {
	var p struct {
		Virtual string `json:"virtual"`
		Domains string `json:"domains"`
		Passwd  string `json:"passwd"`
		UIDs    string `json:"uids"`
		GIDs    string `json:"gids"`
	}
	if err = json.Unmarshal(raw, &p); err != nil {
		return "", "", "", "", "", err
	}
	if strings.ContainsAny(p.Virtual, "\x00") {
		return "", "", "", "", "", fmt.Errorf("NUL in mail map")
	}
	return p.Virtual, p.Domains, p.Passwd, p.UIDs, p.GIDs, nil
}
