package httpserver

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/hosting-panel/panel/internal/configuration"
)

func portalHostnameFromConfig() string {
	if v := strings.TrimSpace(os.Getenv("PANEL_HOSTNAME")); v != "" {
		return v
	}
	dir := strings.TrimSpace(os.Getenv("PANEL_STATE_DIR"))
	if dir == "" {
		dir = "/var/lib/panel"
	}
	body, err := os.ReadFile(filepath.Join(dir, "portal-hostname"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(body))
}

func requestHostname(r *http.Request) string {
	if r == nil {
		return ""
	}
	authority := strings.TrimSpace(r.Host)
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwarded != "" &&
		!strings.Contains(forwarded, ",") && requestFromLoopbackProxy(r) {
		authority = forwarded
	}
	host, _, err := net.SplitHostPort(authority)
	if err != nil {
		host = authority
	}
	return strings.TrimSpace(host)
}

func isLiteralIPHost(host string) bool {
	trimmed := strings.Trim(host, "[]")
	return trimmed != "" && net.ParseIP(trimmed) != nil
}

func isLoopbackHost(host string) bool {
	if host == "" || strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func publicPortalHostname(r *http.Request) string {
	if host := portalHostnameFromConfig(); host != "" && !isLiteralIPHost(host) && !isLoopbackHost(host) {
		return host
	}
	if host := requestHostname(r); host != "" && !isLiteralIPHost(host) && !isLoopbackHost(host) {
		return host
	}
	if host := portalHostnameFromConfig(); host != "" {
		return host
	}
	return requestHostname(r)
}

func controlPortalURL(r *http.Request) string {
	host := publicPortalHostname(r)
	if host == "" || isLoopbackHost(host) {
		return ""
	}
	return fmt.Sprintf("https://%s:%d/", host, configuration.ControlHTTPSPort)
}

func adminToolURL(kind, domain string) string {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return ""
	}
	return fmt.Sprintf("https://%s.%s/", kind, domain)
}
