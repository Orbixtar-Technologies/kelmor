package acme

import "strings"

// HostnamesForPortal returns DNS names issued for the panel hostname certificate.
func HostnamesForPortal(host string) []string {
	host = strings.TrimSpace(host)
	if host == "" || host == "localhost" {
		return nil
	}
	names := []string{host}
	if !strings.HasPrefix(host, "www.") {
		names = append(names, "www."+host)
	}
	return uniqNames(names)
}

// HostnamesForSite returns DNS names for a customer site certificate.
func HostnamesForSite(hostname string, aliases []string, includeToolHosts bool) []string {
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		return nil
	}
	names := []string{hostname}
	names = append(names, aliases...)
	if includeToolHosts {
		// mail.<domain> is the MX target (SMTP/IMAP), not an HTTP-01
		// hostname. Bundling it on the site certificate makes Let's
		// Encrypt fail the whole order when public DNS cannot resolve
		// that name.
		for _, sub := range []string{"www", "webmail", "phpmyadmin"} {
			names = append(names, sub+"."+hostname)
		}
	}
	return uniqNames(names)
}
