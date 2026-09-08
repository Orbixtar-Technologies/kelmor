package operations

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hosting-panel/panel/internal/configuration"
	"github.com/hosting-panel/panel/internal/pkg/validate"
)

func (h *Host) applyWebsite(websiteID, account, domain, docroot, runtime, phpVersion, proxyTarget string, httpsRedirect, enabled, bandwidthHold bool, concurrent int, aliases []string) (Result, error) {
	if _, err := validate.NormalizeDomain(domain); err != nil {
		return Result{}, err
	}
	var cleanAliases []string
	for _, a := range aliases {
		ascii, err := validate.NormalizeDomain(a)
		if err != nil {
			continue
		}
		cleanAliases = append(cleanAliases, ascii)
	}
	switch runtime {
	case "static", "php", "node", "python", "proxy":
	default:
		return Result{}, fmt.Errorf("unsupported runtime")
	}
	if _, err := h.CreateDirectoryTree(docroot, uint32(hostingDirMode(docroot))); err != nil {
		return Result{}, err
	}
	h.hardenWebDocroot(account, docroot)
	spec := configuration.WebsiteSpec{
		WebsiteID: websiteID, Account: account, Domain: domain, DocumentRoot: docroot,
		Runtime: runtime, PHPVersion: phpVersion, ProxyTarget: proxyTarget,
		HTTPSRedirect: httpsRedirect, Revision: 1, Enabled: enabled,
		BandwidthHold: bandwidthHold, ConcurrentWebRequests: concurrent,
		Aliases: cleanAliases,
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
	if err := h.rewriteConnZone(); err != nil {
		return Result{}, err
	}
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

func (h *Host) rewriteConnZone() error {
	dir, err := h.resolve("/etc/nginx/panel-sites")
	if err != nil {
		return err
	}
	hosts := map[string]string{}
	ents, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".conf") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		account, names := parseSiteAccountHosts(string(raw))
		if account == "" {
			continue
		}
		for _, name := range names {
			hosts[name] = account
		}
	}
	_, err = h.ApplyFile("/etc/nginx/conf.d/panel-conn-limit.conf", []byte(configuration.NginxConnZone(hosts)), 0o644)
	return err
}

func parseSiteAccountHosts(conf string) (account string, hosts []string) {
	for _, line := range strings.Split(conf, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "set":
			if len(fields) >= 3 && fields[1] == "$panel_account" {
				account = strings.Trim(fields[2], `";`)
			}
		case "server_name":
			for _, f := range fields[1:] {
				name := strings.TrimSuffix(f, ";")
				if name != "" && name != "_" {
					hosts = append(hosts, name)
				}
			}
		case "root":
			root := strings.TrimSuffix(fields[1], ";")
			if account == "" && strings.HasPrefix(root, "/home/") {
				rest := strings.TrimPrefix(root, "/home/")
				if i := strings.IndexByte(rest, '/'); i > 0 {
					account = rest[:i]
				}
			}
		}
	}
	return account, hosts
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
