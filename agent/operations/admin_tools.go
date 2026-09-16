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
	phpMyAdminRoot          = "/usr/share/phpmyadmin"
	webmailRoot             = "/usr/share/roundcube"
	phpMyAdminOverlayPath   = "/etc/phpmyadmin/conf.d/panel-host.inc.php"
	roundcubeOverlayPath    = "/etc/roundcube/config.panel.inc.php"
	roundcubeMainConfigPath = "/etc/roundcube/config.inc.php"
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
	if err := h.applyAdminToolOverlays(); err != nil {
		return Result{}, err
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
	rootAbs, err := h.systemPath(root)
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
		placeholder := filepath.Join(rootAbs, "index.php")
		if err := os.WriteFile(placeholder, []byte("<?php echo 'panel tool placeholder';\n"), 0o644); err != nil {
			return err
		}
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
	if indexAbs, err := h.systemPath(index); err == nil {
		if _, err := os.Stat(indexAbs); os.IsNotExist(err) {
			return fmt.Errorf("%s index missing", host)
		}
	}
	return nil
}

func (h *Host) applyAdminToolOverlays() error {
	pma := "<?php\n// Managed by Kelmor\n$cfg['AllowArbitraryServer'] = false;\n"
	if _, err := h.ApplyFile(phpMyAdminOverlayPath, []byte(pma), 0o644); err != nil {
		return err
	}
	rc := "<?php\n// Managed by Kelmor\n$config['imap_host'] = 'localhost:143';\n$config['smtp_host'] = 'localhost:587';\n$config['smtp_user'] = '%u';\n$config['smtp_pass'] = '%p';\n"
	if _, err := h.ApplyFile(roundcubeOverlayPath, []byte(rc), 0o644); err != nil {
		return err
	}
	return h.ensureRoundcubeInclude()
}

func (h *Host) ensureRoundcubeInclude() error {
	const needle = "include_once('" + roundcubeOverlayPath + "');"
	abs, err := h.resolve(roundcubeMainConfigPath)
	if err != nil {
		return err
	}
	prev, err := os.ReadFile(abs)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		_, err = h.ApplyFile(roundcubeMainConfigPath, []byte("<?php\n"+needle+"\n"), 0o640)
		return err
	}
	if strings.Contains(string(prev), roundcubeOverlayPath) {
		return nil
	}
	next := strings.TrimRight(string(prev), "\n") + "\n" + needle + "\n"
	_, err = h.ApplyFile(roundcubeMainConfigPath, []byte(next), 0o640)
	return err
}

func (h *Host) retireToolSites(ascii string) {
	ascii = strings.TrimSpace(ascii)
	if ascii == "" || strings.ContainsAny(ascii, "/\\") {
		return
	}
	h.removeManaged("/etc/nginx/panel-sites/phpmyadmin-" + ascii + ".conf")
	h.removeManaged("/etc/nginx/panel-sites/webmail-" + ascii + ".conf")
}

// systemPath maps a host package path into the agent sandbox without
// treating /usr/share as a managed write root.
func (h *Host) systemPath(p string) (string, error) {
	if p == "" || strings.ContainsRune(p, 0) || !filepath.IsAbs(p) {
		return "", fmt.Errorf("invalid system path")
	}
	clean := filepath.Clean(p)
	if strings.Contains(clean, "..") {
		return "", fmt.Errorf("path escape")
	}
	ok := false
	for _, root := range []string{phpMyAdminRoot, webmailRoot} {
		if clean == root || strings.HasPrefix(clean, root+"/") {
			ok = true
			break
		}
	}
	if !ok {
		return "", fmt.Errorf("not a system package path")
	}
	if h.Root == "" {
		return clean, nil
	}
	return filepath.Join(h.Root, strings.TrimPrefix(clean, "/")), nil
}
