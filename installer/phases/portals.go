package phases

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/acme"
	"github.com/hosting-panel/panel/internal/netaddr"
	paneltls "github.com/hosting-panel/panel/internal/tls"
)

func applyPortals(c Config) error {
	if err := ensurePortalCertificate(c); err != nil {
		return err
	}
	if err := installPortalApp(c, "server", "Kelmor Director"); err != nil {
		return err
	}
	if err := installPortalApp(c, "account", "Kelmor Control"); err != nil {
		return err
	}
	writePortalHostnameFile(c)
	tryIssuePortalHostnameCertificate(c)
	if err := writePortalNginx(c, "90-server-portal.conf", 8443, "usr/local/panel/share/portals/server"); err != nil {
		return err
	}
	if err := writePortalNginx(c, "91-account-portal.conf", 8444, "usr/local/panel/share/portals/account"); err != nil {
		return err
	}
	return reloadNginxIfLive(c)
}

func verifyPortals(c Config) error {
	for _, rel := range []string{
		"usr/local/panel/share/portals/server/index.html",
		"usr/local/panel/share/portals/account/index.html",
		"etc/nginx/panel-sites/90-server-portal.conf",
		"etc/nginx/panel-sites/91-account-portal.conf",
	} {
		if _, err := os.Stat(root(c, rel)); err != nil {
			return err
		}
	}
	server, err := os.ReadFile(root(c, "usr/local/panel/share/portals/server/index.html"))
	if err != nil {
		return err
	}
	if !strings.Contains(string(server), "Kelmor Director") {
		return fmt.Errorf("director portal index missing title")
	}
	account, err := os.ReadFile(root(c, "usr/local/panel/share/portals/account/index.html"))
	if err != nil {
		return err
	}
	if !strings.Contains(string(account), "Kelmor Control") {
		return fmt.Errorf("control portal index missing title")
	}
	if !c.Dev {
		if !portalIsBuilt(string(server)) {
			return fmt.Errorf("server portal is not a built SPA (run make portals)")
		}
		if !portalIsBuilt(string(account)) {
			return fmt.Errorf("account portal is not a built SPA (run make portals)")
		}
	}
	if err := verifyPortalTLS(c); err != nil {
		return err
	}
	if c.Dev || installPrefix(c) != "" {
		return nil
	}
	if err := waitListen("127.0.0.1:8443", 2*time.Second); err != nil {
		return fmt.Errorf("kelmor director is not listening: %w", err)
	}
	if err := waitListen("127.0.0.1:8444", 2*time.Second); err != nil {
		return fmt.Errorf("kelmor control is not listening: %w", err)
	}
	return nil
}

func installPortalApp(c Config, name, title string) error {
	dest := root(c, "usr/local/panel/share/portals/"+name)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	src := portalAssetRoot(c, name)
	if src != "" && src != dest {
		if err := copyPortalTree(src, dest); err != nil {
			return err
		}
	}
	if html, err := os.ReadFile(filepath.Join(dest, "index.html")); err == nil && portalIsBuilt(string(html)) {
		return nil
	}
	if !c.Dev {
		return fmt.Errorf("built %s portal assets were not found (run make portals; installer looks next to panel-install under ../share/portals/%s)", name, name)
	}
	body := fmt.Sprintf("<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"><title>%s</title></head><body><h1>%s</h1><p>Built portal assets were not found next to panel-install. Run <code>make portals</code> and re-run the installer.</p></body></html>\n", title, title)
	return os.WriteFile(filepath.Join(dest, "index.html"), []byte(body), 0o644)
}

func portalIsBuilt(html string) bool {
	return strings.Contains(html, "/assets/") && strings.Contains(html, `id="root"`)
}

func verifyPortalTLS(c Config) error {
	for _, rel := range []string{
		"var/lib/panel/certs/panel-portals.crt",
		"var/lib/panel/certs/panel-portals.key",
	} {
		if _, err := os.Stat(root(c, rel)); err != nil {
			return fmt.Errorf("portal TLS material missing: %w", err)
		}
	}
	server, err := os.ReadFile(root(c, "etc/nginx/panel-sites/90-server-portal.conf"))
	if err != nil {
		return err
	}
	account, err := os.ReadFile(root(c, "etc/nginx/panel-sites/91-account-portal.conf"))
	if err != nil {
		return err
	}
	if !strings.Contains(string(server), "listen 8443 ssl") || !strings.Contains(string(server), "ssl_certificate") {
		return fmt.Errorf("server portal nginx is not listening TLS on 8443")
	}
	if !strings.Contains(string(account), "listen 8444 ssl") || !strings.Contains(string(account), "ssl_certificate") {
		return fmt.Errorf("account portal nginx is not listening TLS on 8444")
	}
	if !c.Dev && installPrefix(c) == "" {
		host := strings.TrimSpace(c.Hostname)
		if host != "" && host != "localhost" {
			if _, err := os.Stat(root(c, "var/lib/panel/portal-hostname")); err != nil {
				return fmt.Errorf("portal hostname file missing")
			}
			if readACMEDirectory(c) != "" {
				if _, err := os.Stat(portalHostnameACMEStatusPath(c)); err != nil {
					return fmt.Errorf("portal hostname ACME not attempted")
				}
			}
		}
	}
	return nil
}

func writePortalNginx(c Config, name string, port int, rootRel string) error {
	if err := os.MkdirAll(root(c, "etc/nginx/panel-sites"), 0o755); err != nil {
		return err
	}
	abs := root(c, rootRel)
	if !c.Dev {
		abs = "/" + strings.TrimPrefix(rootRel, "/")
	}
	cert := root(c, "var/lib/panel/certs/panel-portals.crt")
	key := root(c, "var/lib/panel/certs/panel-portals.key")
	if !c.Dev && installPrefix(c) == "" {
		cert = "/var/lib/panel/certs/panel-portals.crt"
		key = "/var/lib/panel/certs/panel-portals.key"
	}
	body := portalNginxServer(port, "_", cert, key, abs)
	if hostCert, hostKey, ok := hostnamePortalCertPaths(c); ok {
		host := strings.TrimSpace(c.Hostname)
		body = portalNginxServer(port, host, hostCert, hostKey, abs) + body
	}
	return os.WriteFile(root(c, "etc/nginx/panel-sites/"+name), []byte(body), 0o644)
}

func portalNginxServer(port int, serverName, cert, key, abs string) string {
	return fmt.Sprintf(`server {
    listen %d ssl;
    listen [::]:%d ssl;
    server_name %s;
    ssl_certificate %s;
    ssl_certificate_key %s;
    ssl_protocols TLSv1.2 TLSv1.3;
    root %s;
    index index.html;
    location /api/ {
        proxy_pass http://127.0.0.1:18080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
    location = /healthz {
        proxy_pass http://127.0.0.1:18080/healthz;
    }
    location / {
        try_files $uri $uri/ /index.html;
    }
}
`, port, port, serverName, cert, key, abs)
}

func hostnamePortalCertPaths(c Config) (cert, key string, ok bool) {
	host := strings.TrimSpace(c.Hostname)
	if host == "" || host == "localhost" {
		return "", "", false
	}
	cert = root(c, "var/lib/panel/certs/"+host+".crt")
	key = root(c, "var/lib/panel/certs/"+host+".key")
	if _, err := os.Stat(cert); err != nil {
		return "", "", false
	}
	if _, err := os.Stat(key); err != nil {
		return "", "", false
	}
	if !c.Dev && installPrefix(c) == "" {
		return "/var/lib/panel/certs/" + host + ".crt", "/var/lib/panel/certs/" + host + ".key", true
	}
	return cert, key, true
}

func portalHostnameACMEStatusPath(c Config) string {
	return root(c, "var/lib/panel/certs/portal-hostname-acme.status")
}

func writePortalHostnameFile(c Config) {
	host := strings.TrimSpace(c.Hostname)
	if host == "" {
		return
	}
	path := root(c, "var/lib/panel/portal-hostname")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, []byte(host+"\n"), 0o644)
}

func tryIssuePortalHostnameCertificate(c Config) {
	if c.Dev || installPrefix(c) != "" {
		return
	}
	host := strings.TrimSpace(c.Hostname)
	if host == "" || host == "localhost" {
		return
	}
	directory := readACMEDirectory(c)
	if directory == "" {
		return
	}
	status := "issued"
	if err := publishPanelHostnameZone(host); err != nil {
		status = "dns: " + err.Error()
	}
	ensureLoopbackHost(host)
	contact := strings.TrimSpace(c.AdminEmail)
	if contact == "" {
		contact = "admin@" + host
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, err := acme.Issue(ctx, &operations.Host{}, host, contact, directory); err != nil {
		if status == "issued" {
			status = "acme: " + err.Error()
		} else {
			status += "; acme: " + err.Error()
		}
	}
	_ = os.WriteFile(portalHostnameACMEStatusPath(c), []byte(status+"\n"), 0o644)
}

func publishPanelHostnameZone(host string) error {
	ip := netaddr.PublicIPv4()
	serial := time.Now().Unix()
	body := fmt.Sprintf("$ORIGIN %s.\n$TTL 3600\n@ IN SOA ns1.%s. hostmaster.%s. (%d 7200 3600 1209600 3600)\n@ IN NS ns1.%s.\n@ 300 IN A %s\nns1 300 IN A %s\nns2 300 IN A %s\n",
		host, host, host, serial, host, ip, ip, ip)
	params, err := json.Marshal(map[string]any{"name": host, "body": body})
	if err != nil {
		return err
	}
	_, err = (&operations.Host{}).Dispatch(context.Background(), operations.Request{
		Method: "ApplyDNSZone",
		Params: params,
	})
	return err
}

func ensurePortalCertificate(c Config) error {
	dir := root(c, "var/lib/panel/certs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	certPath := filepath.Join(dir, "panel-portals.crt")
	keyPath := filepath.Join(dir, "panel-portals.key")
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			return nil
		}
	}
	host := strings.TrimSpace(c.Hostname)
	if host == "" {
		host = "panel.local"
	}
	cert, key, err := paneltls.SelfSignedNames(
		[]string{host, "localhost", "127.0.0.1"},
		time.Now().Add(10*365*24*time.Hour),
	)
	if err != nil {
		return err
	}
	if err := os.WriteFile(certPath, cert, 0o644); err != nil {
		return err
	}
	return os.WriteFile(keyPath, key, 0o600)
}

func portalAssetRoot(c Config, name string) string {
	candidates := []string{
		filepath.Join("dist", "share", "portals", name),
		filepath.Join("portals", name, "dist"),
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append([]string{filepath.Join(filepath.Dir(exe), "..", "share", "portals", name)}, candidates...)
	}
	if !c.Dev && installPrefix(c) == "" {
		candidates = append(candidates, filepath.Join("/usr/local/panel/share/portals", name))
	}
	for _, p := range candidates {
		if _, err := os.Stat(filepath.Join(p, "index.html")); err == nil {
			return p
		}
	}
	return ""
}

func copyPortalTree(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		out := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		f, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(f, in)
		_ = f.Close()
		return copyErr
	})
}

func reloadNginxIfLive(c Config) error {
	if c.Dev || installPrefix(c) != "" {
		return nil
	}
	if _, err := os.Stat("/usr/sbin/nginx"); err != nil {
		return nil
	}
	_ = os.Remove("/etc/nginx/sites-enabled/default")
	if out, err := exec.Command("/usr/sbin/nginx", "-t").CombinedOutput(); err != nil {
		return fmt.Errorf("nginx -t: %s", strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("/usr/sbin/nginx", "-s", "reload").CombinedOutput(); err != nil {
		return fmt.Errorf("nginx reload: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
