package operations

import (
	"fmt"
	"os"
	"strings"

	"github.com/hosting-panel/panel/internal/configuration"
	"github.com/hosting-panel/panel/internal/pkg/validate"
)

func (h *Host) applyWebsite(websiteID, domain, docroot, runtime, phpVersion, proxyTarget string, httpsRedirect bool) (Result, error) {
	if _, err := validate.NormalizeDomain(domain); err != nil {
		return Result{}, err
	}
	switch runtime {
	case "static", "php", "node", "python", "proxy":
	default:
		return Result{}, fmt.Errorf("unsupported runtime")
	}
	if _, err := h.CreateDirectoryTree(docroot, 0o750); err != nil {
		return Result{}, err
	}
	body := configuration.NginxSite(configuration.WebsiteSpec{
		WebsiteID: websiteID, Domain: domain, DocumentRoot: docroot,
		Runtime: runtime, PHPVersion: phpVersion, ProxyTarget: proxyTarget,
		HTTPSRedirect: httpsRedirect, Revision: 1,
	})
	if err := configuration.ValidateNginx(body); err != nil {
		return Result{}, err
	}
	path := "/etc/nginx/panel-sites/" + websiteID + ".conf"
	if _, err := h.ApplyFile(path, []byte(body), 0o644); err != nil {
		return Result{}, err
	}
	if err := h.testNginx(); err != nil {
		return Result{}, err
	}
	index := strings.TrimSuffix(docroot, "/") + "/index.html"
	abs, err := h.resolve(index)
	if err == nil {
		if _, err := os.Stat(abs); os.IsNotExist(err) {
			_ = os.MkdirAll(strings.TrimSuffix(abs, "/index.html"), 0o750)
			_ = os.WriteFile(abs, []byte("<!doctype html><html><body><h1>"+domain+"</h1></body></html>\n"), 0o644)
		}
	}
	return Result{OK: true, Message: "website applied", ObservedState: "active"}, nil
}

func (h *Host) testNginx() error {
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
