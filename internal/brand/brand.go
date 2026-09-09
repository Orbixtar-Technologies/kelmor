package brand

import "strings"

// Product identity. kelmor.host is the product domain only — never a
// default customer hosting zone or tenant FQDN.
const (
	Product  = "Kelmor"
	Director = "Kelmor Director"
	Control  = "Kelmor Control"
	Agent    = "Kelmor Agent"
	APITitle = "Kelmor Control Plane API"
)

// LegacyChromeNames are retired product titles. They must not appear in
// user-visible portal HTML, login chrome, OpenAPI UI, or installer banners.
// On-disk privilege paths (/var/lib/panel, panel user, PANEL_*) stay.
var LegacyChromeNames = []string{
	"Hosting Panel",
	"Server Portal",
	"Account Portal",
}

// ContainsLegacyChrome reports the first retired product title in s.
func ContainsLegacyChrome(s string) string {
	for _, name := range LegacyChromeNames {
		if strings.Contains(s, name) {
			return name
		}
	}
	return ""
}

// ReservedUsernames are Linux / panel identities that tenants cannot claim.
var ReservedUsernames = []string{
	"kelmor",
	"kelmor-agent",
	"kelmor-api",
	"kelmor-worker",
}

// BinaryAlias maps a legacy panel-* name to the Kelmor binary name.
func BinaryAlias(legacy string) string {
	switch legacy {
	case "panel-api":
		return "kelmor-api"
	case "panel-worker":
		return "kelmor-worker"
	case "panel-agent":
		return "kelmor-agent"
	case "panel-cli":
		return "kelmor-cli"
	case "panel-install":
		return "kelmor-install"
	case "panel-dev":
		return "kelmor-dev"
	case "panel-updater":
		return "kelmor-updater"
	case "panel-backup":
		return "kelmor-backup"
	case "panel-smtp-policy":
		return "kelmor-smtp-policy"
	case "panel-object-store":
		return "kelmor-object-store"
	default:
		return legacy
	}
}

// ControlPlaneService reports whether name is an unprivileged API or worker
// process (privilege zone A). Legacy and Kelmor names both count.
func ControlPlaneService(name string) bool {
	switch name {
	case "panel-api", "kelmor-api", "panel-worker", "kelmor-worker":
		return true
	default:
		return false
	}
}
