package phases

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	paneltls "github.com/hosting-panel/panel/internal/tls"
)

func applyPortals(c Config) error {
	if err := ensurePortalCertificate(c); err != nil {
		return err
	}
	if err := installPortalApp(c, "server", "Server Portal"); err != nil {
		return err
	}
	if err := installPortalApp(c, "account", "Account Portal"); err != nil {
		return err
	}
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
	if !strings.Contains(string(server), "Server Portal") {
		return fmt.Errorf("server portal index missing title")
	}
	account, err := os.ReadFile(root(c, "usr/local/panel/share/portals/account/index.html"))
	if err != nil {
		return err
	}
	if !strings.Contains(string(account), "Account Portal") {
		return fmt.Errorf("account portal index missing title")
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
	if c.Dev {
		return nil
	}
	if err := waitListen("127.0.0.1:8443", 2*time.Second); err != nil {
		return fmt.Errorf("server portal is not listening: %w", err)
	}
	if err := waitListen("127.0.0.1:8444", 2*time.Second); err != nil {
		return fmt.Errorf("account portal is not listening: %w", err)
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
	body := fmt.Sprintf(`server {
    listen %d ssl;
    listen [::]:%d ssl;
    server_name _;
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
`, port, port, cert, key, abs)
	return os.WriteFile(root(c, "etc/nginx/panel-sites/"+name), []byte(body), 0o644)
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
