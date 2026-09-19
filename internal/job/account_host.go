package job

import (
	"context"
	"net"
	"path/filepath"
	"strings"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/configuration"
	"github.com/hosting-panel/panel/internal/store"
)

func accountShellPath(class string) string {
	switch strings.TrimSpace(class) {
	case "jailed":
		return "/usr/sbin/rssh"
	default:
		return "/usr/sbin/nologin"
	}
}

func accountPublishIPv4(acc *store.Account) string {
	if ip := parseIPv4(accountIP(acc)); ip != "" {
		return ip
	}
	return publicIPv4()
}

func accountPublishIPv6(acc *store.Account) string {
	return parseIPv6(accountIP(acc))
}

func accountIP(acc *store.Account) string {
	if acc == nil {
		return ""
	}
	return strings.TrimSpace(acc.IPAddress)
}

func parseIPv4(s string) string {
	ip := net.ParseIP(strings.TrimSpace(s))
	if ip == nil || ip.To4() == nil {
		return ""
	}
	return ip.To4().String()
}

func parseIPv6(s string) string {
	ip := net.ParseIP(strings.TrimSpace(s))
	if ip == nil || ip.To4() != nil {
		return ""
	}
	return ip.String()
}

func managedDNSName(name string) bool {
	switch strings.TrimSpace(name) {
	case "@", "www":
		return true
	}
	for _, service := range configuration.AccountServiceHostnames() {
		if name == service {
			return true
		}
	}
	return false
}

func restorePathPrefix(raw string) string {
	prefix := filepath.Clean(strings.TrimSpace(raw))
	prefix = strings.TrimPrefix(prefix, "/")
	if prefix == "." || prefix == "" || strings.Contains(prefix, "..") {
		return ""
	}
	return prefix
}

func (w *Worker) applyAccountShell(acc *store.Account) error {
	if acc == nil || w.Agent == nil {
		return nil
	}
	_, err := w.Agent.Dispatch(context.Background(), operations.Request{
		Method: "SetLinuxShell",
		Params: mustJSON(map[string]any{
			"username": acc.Username,
			"shell":    accountShellPath(acc.ShellClass),
		}),
	})
	return err
}
