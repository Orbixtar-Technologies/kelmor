package operations

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hosting-panel/panel/internal/configuration"
	"github.com/hosting-panel/panel/internal/pkg/validate"
)

func (h *Host) applyWebsite(websiteID, account, domain, docroot, runtime, phpVersion, proxyTarget string, httpsRedirect, enabled, bandwidthHold bool) (Result, error) {
	if _, err := validate.NormalizeDomain(domain); err != nil {
		return Result{}, err
	}
	switch runtime {
	case "static", "php", "node", "python", "proxy":
	default:
		return Result{}, fmt.Errorf("unsupported runtime")
	}
	if _, err := h.CreateDirectoryTree(docroot, uint32(hostingDirMode(docroot))); err != nil {
		return Result{}, err
	}
	spec := configuration.WebsiteSpec{
		WebsiteID: websiteID, Account: account, Domain: domain, DocumentRoot: docroot,
		Runtime: runtime, PHPVersion: phpVersion, ProxyTarget: proxyTarget,
		HTTPSRedirect: httpsRedirect, Revision: 1, Enabled: enabled,
		BandwidthHold: bandwidthHold,
	}
	if certAbs, err := h.resolve("/var/lib/panel/certs/" + domain + ".crt"); err == nil {
		if _, err := os.Stat(certAbs); err == nil {
			spec.TLSCert = "/var/lib/panel/certs/" + domain + ".crt"
			spec.TLSKey = "/var/lib/panel/certs/" + domain + ".key"
		}
	}
	body := configuration.NginxSite(spec)
	if err := configuration.ValidateNginx(body); err != nil {
		return Result{}, err
	}
	path := "/etc/nginx/panel-sites/" + websiteID + ".conf"
	if _, err := h.ApplyFile(path, []byte(body), 0o644); err != nil {
		return Result{}, err
	}
	h.retireOtherSites(domain, path)
	if err := h.testNginx(); err != nil {
		return Result{}, err
	}
	if h.live() {
		_, _ = runFixed("/usr/sbin/nginx", "-s", "reload")
	}
	index := strings.TrimSuffix(docroot, "/") + "/index.html"
	abs, err := h.resolve(index)
	if err == nil {
		if _, err := os.Stat(abs); os.IsNotExist(err) {
			_ = os.MkdirAll(strings.TrimSuffix(abs, "/index.html"), 0o750)
			_, _ = h.ApplyFile(index, []byte("<!doctype html><html><body><h1>"+domain+"</h1></body></html>\n"), 0o640)
		} else {
			_ = os.Chmod(abs, 0o640)
			h.chownAccountPath(abs)
		}
	}
	if runtime == "php" {
		php := strings.TrimSuffix(docroot, "/") + "/index.php"
		if pabs, err := h.resolve(php); err == nil {
			if _, err := os.Stat(pabs); os.IsNotExist(err) {
				_, _ = h.ApplyFile(php, []byte("<?php header('content-type: text/plain'); echo 'php '.PHP_VERSION.' '.get_current_user().\"\\n\";\n"), 0o640)
			} else {
				_ = os.Chmod(pabs, 0o640)
				h.chownAccountPath(pabs)
			}
		}
	}
	return Result{OK: true, Message: "website applied", ObservedState: "active"}, nil
}

func (h *Host) retireOtherSites(domain, keep string) {
	if domain == "" {
		return
	}
	dir := "/etc/nginx/panel-sites"
	if abs, err := h.resolve(dir); err == nil {
		dir = abs
	}
	keepAbs := keep
	if p, err := h.resolve(keep); err == nil {
		keepAbs = p
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	needle := "server_name " + domain + ";"
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if p == keepAbs {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if strings.Contains(string(b), needle) {
			_ = os.Remove(p)
		}
	}
}

func (h *Host) testNginx() error {
	if !h.live() {
		return nil
	}
	candidates := []string{"/usr/sbin/nginx", "/usr/bin/nginx"}
	for _, bin := range candidates {
		if _, err := os.Stat(bin); err != nil {
			continue
		}
		out, err := runFixed(bin, "-t")
		if err != nil {
			return fmt.Errorf("nginx -t: %s", strings.TrimSpace(string(out)))
		}
		return nil
	}
	return nil
}
