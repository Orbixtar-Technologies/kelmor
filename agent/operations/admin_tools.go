package operations

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hosting-panel/panel/internal/configuration"
	"github.com/hosting-panel/panel/internal/pkg/validate"
)

const (
	phpMyAdminRoot = "/usr/share/phpmyadmin"
	webmailRoot    = "/usr/share/roundcube"
)

func (h *Host) applyAdminTools(domain string, tools []string) (Result, error) {
	ascii, err := validate.NormalizeDomain(domain)
	if err != nil {
		return Result{}, err
	}
	cert := "/var/lib/panel/certs/" + ascii + ".crt"
	key := "/var/lib/panel/certs/" + ascii + ".key"
	if certAbs, err := h.resolve(cert); err == nil {
		if _, err := os.Stat(certAbs); err != nil {
			cert = ""
			key = ""
		}
	}
	wanted := map[string]bool{}
	if len(tools) == 0 {
		wanted["phpmyadmin"] = true
		wanted["roundcube"] = true
	} else {
		for _, tool := range tools {
			switch tool {
			case "phpmyadmin", "roundcube":
				wanted[tool] = true
			}
		}
	}
	catalog := []struct {
		id, host, root, key string
	}{
		{"phpmyadmin-" + ascii, "phpmyadmin." + ascii, phpMyAdminRoot, "phpmyadmin"},
		{"webmail-" + ascii, "webmail." + ascii, webmailRoot, "roundcube"},
	}
	for _, tool := range catalog {
		if !wanted[tool.key] {
			continue
		}
		if err := h.applyToolSite(tool.id, tool.host, tool.root, cert, key); err != nil {
			return Result{}, err
		}
	}
	if h.live() {
		if err := h.testNginx(); err != nil {
			return Result{}, err
		}
		_, _ = runFixed("/usr/sbin/nginx", "-s", "reload")
	}
	return Result{OK: true, Message: "admin tools applied for " + ascii, ObservedState: "active"}, nil
}

func (h *Host) applyToolSite(id, host, root, cert, key string) error {
	rootAbs, err := h.resolve(root)
	if err != nil {
		return err
	}
	if _, err := os.Stat(rootAbs); err != nil {
		if h.Root == "" {
			return fmt.Errorf("%s is not installed on this host", filepath.Base(root))
		}
		if err := os.MkdirAll(rootAbs, 0o755); err != nil {
			return err
		}
		_ = os.WriteFile(filepath.Join(rootAbs, "index.php"), []byte("<?php echo 'panel tool placeholder';\n"), 0o644)
	}
	body := configuration.NginxToolSite(configuration.ToolSiteSpec{
		SiteID: id, Hostname: host, DocumentRoot: root, TLSCert: cert, TLSKey: key,
	})
	if err := configuration.ValidateNginx(body); err != nil {
		return err
	}
	path := "/etc/nginx/panel-sites/" + id + ".conf"
	if _, err := h.ApplyFile(path, []byte(body), 0o644); err != nil {
		return err
	}
	index := strings.TrimSuffix(root, "/") + "/index.php"
	if indexAbs, err := h.resolve(index); err == nil {
		if _, err := os.Stat(indexAbs); os.IsNotExist(err) {
			return fmt.Errorf("%s index missing", host)
		}
	}
	return nil
}
