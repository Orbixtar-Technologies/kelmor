package netaddr

import (
	"net"
	"os"
	"strings"
	"time"
)

// PublicIPv4 is the address published in DNS A records and bound by
// PowerDNS. PANEL_PUBLIC_IPV4 wins; otherwise the UDP source address
// toward 1.1.1.1 is used. Loopback is the last-resort lab default.
func PublicIPv4() string {
	if v := strings.TrimSpace(os.Getenv("PANEL_PUBLIC_IPV4")); v != "" {
		if ip := net.ParseIP(v); ip != nil && ip.To4() != nil {
			return ip.To4().String()
		}
	}
	c, err := net.DialTimeout("udp", "1.1.1.1:53", 400*time.Millisecond)
	if err == nil {
		defer c.Close()
		if addr, ok := c.LocalAddr().(*net.UDPAddr); ok && addr.IP.To4() != nil && !addr.IP.IsLoopback() {
			return addr.IP.To4().String()
		}
	}
	return "127.0.0.1"
}

// DNSListenIPv4 is 127.0.0.1 plus PublicIPv4 when that address is
// non-loopback, so the resolver stays reachable locally and on the NIC.
func DNSListenIPv4() []string {
	pub := PublicIPv4()
	if pub == "127.0.0.1" {
		return []string{"127.0.0.1"}
	}
	return []string{"127.0.0.1", pub}
}
