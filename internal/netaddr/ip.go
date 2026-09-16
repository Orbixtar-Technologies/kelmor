package netaddr

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var lookupExternalIPv4 = lookupExternalIPv4Default

// PublicIPv4 is the address published in DNS A records and FTP PASV.
// PANEL_PUBLIC_IPV4 wins when it is a globally routable IPv4. Private
// NIC addresses (Aliyun/AWS NAT, RFC1918) are skipped so Let's Encrypt
// does not see "no valid A records". Next: UDP source toward 1.1.1.1,
// then public.env, then cloud metadata / HTTPS echo. Loopback is the
// last-resort lab default. PowerDNS still binds the local NIC via
// DNSListenIPv4.
func PublicIPv4() string {
	for _, candidate := range []string{
		strings.TrimSpace(os.Getenv("PANEL_PUBLIC_IPV4")),
		udpSourceIPv4(),
		readPublicEnvFile(),
	} {
		if ip := publishableIPv4(candidate); ip != "" {
			return ip
		}
	}
	if ip := publishableIPv4(lookupExternalIPv4()); ip != "" {
		return ip
	}
	return "127.0.0.1"
}

func udpSourceIPv4() string {
	c, err := net.DialTimeout("udp", "1.1.1.1:53", 400*time.Millisecond)
	if err != nil {
		return ""
	}
	defer c.Close()
	addr, ok := c.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP.To4() == nil || addr.IP.IsLoopback() {
		return ""
	}
	return addr.IP.To4().String()
}

func publishableIPv4(s string) string {
	ip := net.ParseIP(strings.TrimSpace(s))
	if ip == nil || ip.To4() == nil || IsPrivateIPv4(ip) {
		return ""
	}
	return ip.To4().String()
}

// IsPrivateIPv4 reports loopback, link-local, unspecified, multicast,
// RFC1918, and CGNAT addresses that Let's Encrypt will not accept.
func IsPrivateIPv4(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	return ip4.IsPrivate() || ip4.IsLoopback() || ip4.IsLinkLocalUnicast() ||
		ip4.IsUnspecified() || ip4.IsMulticast()
}

func IsPrivateIPv4String(s string) bool {
	ip := net.ParseIP(strings.TrimSpace(s))
	return ip != nil && IsPrivateIPv4(ip)
}

// RewritePrivateIP4Tokens replaces ip4:<private> tokens in SPF-style
// text with ip4:<pubIP>.
func RewritePrivateIP4Tokens(content, pubIP string) string {
	if pubIP == "" || IsPrivateIPv4String(pubIP) {
		return content
	}
	const prefix = "ip4:"
	out := content
	start := 0
	for {
		rel := strings.Index(out[start:], prefix)
		if rel < 0 {
			return out
		}
		i := start + rel + len(prefix)
		j := i
		for j < len(out) && (out[j] == '.' || (out[j] >= '0' && out[j] <= '9')) {
			j++
		}
		token := out[i:j]
		if IsPrivateIPv4String(token) && token != pubIP {
			out = out[:i] + pubIP + out[j:]
			start = i + len(pubIP)
			continue
		}
		start = j
	}
}

func lookupExternalIPv4Default() string {
	client := &http.Client{Timeout: 2 * time.Second}
	for _, raw := range []string{
		"http://100.100.100.200/latest/meta-data/eipv4",
		"http://100.100.100.200/latest/meta-data/public-ipv4",
		"https://api.ipify.org",
		"https://ipv4.icanhazip.com",
	} {
		req, err := http.NewRequest(http.MethodGet, raw, nil)
		if err != nil {
			continue
		}
		res, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(res.Body, 64))
		_ = res.Body.Close()
		if res.StatusCode != http.StatusOK {
			continue
		}
		if ip := publishableIPv4(string(body)); ip != "" {
			return ip
		}
	}
	return ""
}

func publicEnvPath() string {
	if p := strings.TrimSpace(os.Getenv("PANEL_PUBLIC_ENV")); p != "" {
		return p
	}
	if d := strings.TrimSpace(os.Getenv("PANEL_STATE_DIR")); d != "" {
		return filepath.Join(d, "public.env")
	}
	return "/var/lib/panel/public.env"
}

func readPublicEnvFile() string {
	f, err := os.Open(publicEnvPath())
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(k) != "PANEL_PUBLIC_IPV4" {
			continue
		}
		if ip := net.ParseIP(strings.TrimSpace(v)); ip != nil && ip.To4() != nil {
			return ip.To4().String()
		}
	}
	return ""
}

// DNSListenIPv4 is 127.0.0.1 plus an address PowerDNS can bind on the
// host. PublicIPv4 is used when it is assigned locally; otherwise the
// primary non-loopback NIC address is used for NAT/cloud deployments.
func DNSListenIPv4() []string {
	addrs := []string{"127.0.0.1"}
	pub := PublicIPv4()
	if pub == "127.0.0.1" {
		return addrs
	}
	if isAssignedIPv4(pub) {
		return append(addrs, pub)
	}
	if nic := primaryInterfaceIPv4(); nic != "" {
		return append(addrs, nic)
	}
	return addrs
}

func isAssignedIPv4(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() == nil {
		return false
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var host net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				host = v.IP
			case *net.IPAddr:
				host = v.IP
			default:
				continue
			}
			if host.To4() != nil && host.Equal(parsed) {
				return true
			}
		}
	}
	return false
}

func primaryInterfaceIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var host net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				host = v.IP
			case *net.IPAddr:
				host = v.IP
			default:
				continue
			}
			if v4 := host.To4(); v4 != nil && !v4.IsLoopback() {
				return v4.String()
			}
		}
	}
	return ""
}
