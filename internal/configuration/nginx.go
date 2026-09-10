package configuration

import (
	"fmt"
	"strings"
)

type WebsiteSpec struct {
	WebsiteID             string
	Account               string
	Domain                string
	DocumentRoot          string
	Runtime               string
	PHPVersion            string
	ProxyTarget           string
	HTTPSRedirect         bool
	Revision              int64
	TLSCert               string
	TLSKey                string
	Enabled               bool
	BandwidthHold         bool
	ConcurrentWebRequests int
	Aliases               []string
}

func NginxSite(s WebsiteSpec) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Managed by Kelmor\n# resource: %s\n# revision: %d\n# template: nginx/%s-site/v3\n# DO NOT EDIT\n", s.WebsiteID, s.Revision, s.Runtime)
	if !s.Enabled {
		b.WriteString(limitedServer("80", s, 503, "account suspended\\n"))
		if s.TLSCert != "" && s.TLSKey != "" {
			b.WriteString(limitedServer("443 ssl", s, 503, "account suspended\\n"))
		}
		return b.String()
	}
	if s.BandwidthHold {
		b.WriteString(limitedServer("80", s, 509, "bandwidth limit exceeded\\n"))
		if s.TLSCert != "" && s.TLSKey != "" {
			b.WriteString(limitedServer("443 ssl", s, 509, "bandwidth limit exceeded\\n"))
		}
		return b.String()
	}
	b.WriteString("server {\n")
	b.WriteString("    listen 80;\n")
	b.WriteString("    listen [::]:80;\n")
	fmt.Fprintf(&b, "    server_name %s;\n", serverNames(s))
	fmt.Fprintf(&b, "    root %s;\n", s.DocumentRoot)
	b.WriteString("    index index.php index.html;\n")
	fmt.Fprintf(&b, "    access_log /var/log/nginx/%s.access.log;\n", s.WebsiteID)
	fmt.Fprintf(&b, "    error_log /var/log/nginx/%s.error.log;\n", s.WebsiteID)
	b.WriteString(connLimitLines(s))
	b.WriteString("    location ^~ /.well-known/acme-challenge/ { root /var/lib/panel/acme-www; default_type text/plain; }\n")
	if s.HTTPSRedirect && s.TLSCert != "" {
		b.WriteString("    if ($scheme = http) { return 301 https://$host$request_uri; }\n")
	}
	switch s.Runtime {
	case "php":
		sock := s.Account
		if sock == "" {
			sock = s.WebsiteID
		}
		b.WriteString("    location / { try_files $uri $uri/ /index.php?$query_string; }\n")
		fmt.Fprintf(&b, "    location ~ \\.php$ { include fastcgi_params; fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name; fastcgi_pass unix:/run/php/panel-%s.sock; }\n", sock)
	case "node", "python":
		fmt.Fprintf(&b, "    location / { proxy_pass http://unix:/run/panel/apps/%s.sock; proxy_set_header Host $host; }\n", s.WebsiteID)
	case "proxy":
		if s.ProxyTarget != "" {
			fmt.Fprintf(&b, "    location / { proxy_pass %s; proxy_set_header Host $host; }\n", s.ProxyTarget)
		}
	default:
		b.WriteString("    location / { try_files $uri $uri/ =404; }\n")
	}
	b.WriteString("}\n")
	if s.TLSCert != "" && s.TLSKey != "" {
		b.WriteString("server {\n")
		b.WriteString("    listen 443 ssl;\n")
		b.WriteString("    listen [::]:443 ssl;\n")
		fmt.Fprintf(&b, "    server_name %s;\n", serverNames(s))
		fmt.Fprintf(&b, "    root %s;\n", s.DocumentRoot)
		b.WriteString("    index index.php index.html;\n")
		fmt.Fprintf(&b, "    ssl_certificate %s;\n", s.TLSCert)
		fmt.Fprintf(&b, "    ssl_certificate_key %s;\n", s.TLSKey)
		b.WriteString(connLimitLines(s))
		b.WriteString("    location ^~ /.well-known/acme-challenge/ { root /var/lib/panel/acme-www; default_type text/plain; }\n")
		switch s.Runtime {
		case "php":
			sock := s.Account
			if sock == "" {
				sock = s.WebsiteID
			}
			b.WriteString("    location / { try_files $uri $uri/ /index.php?$query_string; }\n")
			fmt.Fprintf(&b, "    location ~ \\.php$ { include fastcgi_params; fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name; fastcgi_pass unix:/run/php/panel-%s.sock; }\n", sock)
		case "node", "python":
			fmt.Fprintf(&b, "    location / { proxy_pass http://unix:/run/panel/apps/%s.sock; proxy_set_header Host $host; }\n", s.WebsiteID)
		case "proxy":
			if s.ProxyTarget != "" {
				fmt.Fprintf(&b, "    location / { proxy_pass %s; proxy_set_header Host $host; }\n", s.ProxyTarget)
			}
		default:
			b.WriteString("    location / { try_files $uri $uri/ =404; }\n")
		}
		b.WriteString("}\n")
	}
	return b.String()
}

func serverNames(s WebsiteSpec) string {
	names := []string{s.Domain}
	seen := map[string]bool{s.Domain: true}
	for _, a := range s.Aliases {
		a = strings.TrimSpace(a)
		if a == "" || seen[a] || strings.ContainsAny(a, " \t\n{};") {
			continue
		}
		seen[a] = true
		names = append(names, a)
	}
	return strings.Join(names, " ")
}

func connLimitLines(s WebsiteSpec) string {
	if s.Account == "" || s.ConcurrentWebRequests < 1 {
		return ""
	}
	return fmt.Sprintf("    set $panel_account %q;\n    limit_conn panel_acct %d;\n    limit_conn_status 429;\n", s.Account, s.ConcurrentWebRequests)
}

func NginxConnZone(hosts map[string]string) string {
	var b strings.Builder
	b.WriteString("map $host $panel_account {\n    default \"\";\n")
	for host, account := range hosts {
		host = strings.TrimSpace(host)
		account = strings.TrimSpace(account)
		if host == "" || account == "" || strings.ContainsAny(host+account, " \t\n{};") {
			continue
		}
		fmt.Fprintf(&b, "    %s %s;\n", host, account)
	}
	b.WriteString("}\nlimit_conn_zone $panel_account zone=panel_acct:10m;\n")
	return b.String()
}

func limitedServer(listen string, s WebsiteSpec, status int, body string) string {
	var b strings.Builder
	b.WriteString("server {\n")
	fmt.Fprintf(&b, "    listen %s;\n", listen)
	if strings.HasPrefix(listen, "80") {
		b.WriteString("    listen [::]:80;\n")
	} else {
		b.WriteString("    listen [::]:443 ssl;\n")
	}
	fmt.Fprintf(&b, "    server_name %s;\n", serverNames(s))
	fmt.Fprintf(&b, "    access_log /var/log/nginx/%s.access.log;\n", s.WebsiteID)
	if strings.Contains(listen, "ssl") && s.TLSCert != "" {
		fmt.Fprintf(&b, "    ssl_certificate %s;\n", s.TLSCert)
		fmt.Fprintf(&b, "    ssl_certificate_key %s;\n", s.TLSKey)
	}
	b.WriteString("    location ^~ /.well-known/acme-challenge/ { root /var/lib/panel/acme-www; default_type text/plain; }\n")
	fmt.Fprintf(&b, "    location / { default_type text/plain; return %d '%s'; }\n", status, body)
	b.WriteString("}\n")
	return b.String()
}

func ValidateNginx(conf string) error {
	if strings.Contains(conf, "\x00") {
		return fmt.Errorf("NUL in configuration")
	}
	if !strings.Contains(conf, "server {") || !strings.Contains(conf, "server_name") {
		return fmt.Errorf("nginx configuration missing server block")
	}
	open := strings.Count(conf, "{")
	close := strings.Count(conf, "}")
	if open != close {
		return fmt.Errorf("unbalanced nginx braces")
	}
	return nil
}

func PHPPool(account, version string, maxChildren int) string {
	return PHPPoolFor(account, account, version, maxChildren)
}

func PHPPoolFor(account, group, version string, maxChildren int) string {
	if maxChildren < 1 {
		maxChildren = 5
	}
	if group == "" {
		group = account
	}
	return fmt.Sprintf(`[panel-%s]
user = %s
group = %s
listen = /run/php/panel-%s.sock
listen.owner = www-data
listen.group = www-data
pm = ondemand
pm.max_children = %d
php_admin_value[open_basedir] = /home/%s:/tmp:/usr/share/php
`, account, account, group, account, maxChildren, account)
}

const defaultDirectorHTTPSPort = 8443

// PortalHTTPRedirect returns nginx :80 vhosts for the panel hostname. Each
// block serves HTTP-01 challenges and redirects browsers to the Director HTTPS
// port (default 8443).
func PortalHTTPRedirect(hostname string, directorPort int) string {
	hostname = strings.TrimSpace(hostname)
	if hostname == "" || hostname == "localhost" {
		return ""
	}
	if directorPort < 1 {
		directorPort = defaultDirectorHTTPSPort
	}
	var b strings.Builder
	writePortalHTTPBlock(&b, []string{hostname, "www." + hostname}, hostname, directorPort)
	if strings.HasSuffix(hostname, ".kelmor.host") && hostname != "kelmor.host" {
		writePortalHTTPBlock(&b, []string{"kelmor.host", "www.kelmor.host"}, hostname, directorPort)
	}
	return b.String()
}

func writePortalHTTPBlock(b *strings.Builder, serverNames []string, redirectHost string, directorPort int) {
	b.WriteString("server {\n")
	b.WriteString("    listen 80;\n")
	b.WriteString("    listen [::]:80;\n")
	fmt.Fprintf(b, "    server_name %s;\n", strings.Join(serverNames, " "))
	b.WriteString("    location ^~ /.well-known/acme-challenge/ { root /var/lib/panel/acme-www; default_type text/plain; }\n")
	fmt.Fprintf(b, "    location / { return 301 https://%s:%d$request_uri; }\n", redirectHost, directorPort)
	b.WriteString("}\n")
}

type ToolSiteSpec struct {
	SiteID       string
	Hostname     string
	DocumentRoot string
	TLSCert      string
	TLSKey       string
}

func NginxToolSite(s ToolSiteSpec) string {
	var b strings.Builder
	writeToolServer(&b, "80", s, false)
	if s.TLSCert != "" && s.TLSKey != "" {
		writeToolServer(&b, "443 ssl", s, true)
	}
	return b.String()
}

func writeToolServer(b *strings.Builder, listen string, s ToolSiteSpec, tls bool) {
	b.WriteString("server {\n")
	fmt.Fprintf(b, "    listen %s;\n", listen)
	if strings.HasPrefix(listen, "80") {
		b.WriteString("    listen [::]:80;\n")
	} else {
		b.WriteString("    listen [::]:443 ssl;\n")
	}
	fmt.Fprintf(b, "    server_name %s;\n", s.Hostname)
	fmt.Fprintf(b, "    root %s;\n", s.DocumentRoot)
	b.WriteString("    index index.php index.html;\n")
	fmt.Fprintf(b, "    access_log /var/log/nginx/%s.access.log;\n", s.SiteID)
	fmt.Fprintf(b, "    error_log /var/log/nginx/%s.error.log;\n", s.SiteID)
	b.WriteString("    location ^~ /.well-known/acme-challenge/ { root /var/lib/panel/acme-www; default_type text/plain; }\n")
	if tls {
		fmt.Fprintf(b, "    ssl_certificate %s;\n", s.TLSCert)
		fmt.Fprintf(b, "    ssl_certificate_key %s;\n", s.TLSKey)
	}
	b.WriteString("    location / { try_files $uri $uri/ /index.php?$query_string; }\n")
	b.WriteString("    location ~ \\.php$ { include fastcgi_params; fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name; fastcgi_pass unix:/run/php/php8.3-fpm.sock; }\n")
	b.WriteString("}\n")
}

func SystemdSlice(username string, cpuPercent int, memoryBytes int64, tasksMax, ioWeight, iops int) string {
	if tasksMax < 1 {
		tasksMax = 100
	}
	if ioWeight < 1 {
		ioWeight = 100
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[Slice]\nCPUQuota=%d%%\nMemoryMax=%d\nTasksMax=%d\nIOWeight=%d\n", cpuPercent, memoryBytes, tasksMax, ioWeight)
	if iops > 0 {
		fmt.Fprintf(&b, "# panel_iops=%d\n", iops)
	}
	_ = username
	return b.String()
}
