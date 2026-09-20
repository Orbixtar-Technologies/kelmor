package operations

import (
	"fmt"
	"strings"
	"unicode"
)

const maxFTPBannerLen = 512

func (h *Host) applyFTPServerConfig(spec HostConfigSpec) error {
	banner, err := sanitizeFTPBanner(spec.FTPBanner)
	if err != nil {
		return err
	}
	pasvMin := spec.PasvMin
	pasvMax := spec.PasvMax
	if pasvMin < 1 {
		pasvMin = 40000
	}
	if pasvMax < pasvMin || pasvMax > 65535 {
		return fmt.Errorf("ftp PASV range is invalid")
	}
	listen := "NO"
	if spec.FTPEnabled {
		listen = "YES"
	}
	overlay := fmt.Sprintf("pasv_min_port=%d\npasv_max_port=%d\nlisten=%s\n", pasvMin, pasvMax, listen)
	if banner != "" {
		overlay += "ftpd_banner=" + banner + "\n"
	}
	existing, err := h.readManaged("/etc/vsftpd.conf", 1<<20)
	if err != nil {
		return fmt.Errorf("vsftpd configuration is not present")
	}
	if _, err := h.ApplyFile("/etc/panel/vsftpd-panel.conf", []byte(overlay), 0o644); err != nil {
		return err
	}
	merged := upsertVsftpdKeys(string(existing), map[string]string{
		"pasv_min_port": fmt.Sprintf("%d", pasvMin),
		"pasv_max_port": fmt.Sprintf("%d", pasvMax),
		"listen":        listen,
		"ftpd_banner":   banner,
	})
	if _, err := h.ApplyFile("/etc/vsftpd.conf", []byte(merged), 0o644); err != nil {
		return err
	}
	if h.live() {
		if spec.FTPEnabled {
			_ = h.ensureVsftpd()
		} else {
			_ = controlNamedService("vsftpd", "stop")
		}
	}
	return nil
}

func sanitizeFTPBanner(raw string) (string, error) {
	banner := strings.TrimSpace(raw)
	if banner == "" {
		return "", nil
	}
	if strings.ContainsAny(banner, "\n\r\x00") {
		return "", fmt.Errorf("ftp banner must be a single line")
	}
	if len(banner) > maxFTPBannerLen {
		return "", fmt.Errorf("ftp banner is too long")
	}
	for _, r := range banner {
		if r == '\t' {
			continue
		}
		if unicode.IsControl(r) {
			return "", fmt.Errorf("ftp banner contains control characters")
		}
	}
	return banner, nil
}

func upsertVsftpdKeys(existing string, keys map[string]string) string {
	seen := map[string]bool{}
	var b strings.Builder
	for _, line := range strings.Split(existing, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if existing != "" {
				b.WriteByte('\n')
			}
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			b.WriteString(line)
			b.WriteByte('\n')
			continue
		}
		key, _, ok := strings.Cut(trimmed, "=")
		key = strings.TrimSpace(key)
		if !ok {
			b.WriteString(line)
			b.WriteByte('\n')
			continue
		}
		if key == "banner_file" {
			b.WriteString("# ")
			b.WriteString(trimmed)
			b.WriteString(" # Kelmor uses ftpd_banner\n")
			continue
		}
		value, managed := keys[key]
		if !managed {
			b.WriteString(line)
			b.WriteByte('\n')
			continue
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		if key == "ftpd_banner" && value == "" {
			continue
		}
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(value)
		b.WriteByte('\n')
	}
	for _, key := range []string{"pasv_min_port", "pasv_max_port", "listen", "ftpd_banner"} {
		value, ok := keys[key]
		if !ok || seen[key] {
			continue
		}
		if key == "ftpd_banner" && value == "" {
			continue
		}
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(value)
		b.WriteByte('\n')
	}
	return b.String()
}
