package acme

import (
	"context"
	"fmt"
	"net"
	"time"
)

var (
	publicDNSServers  = []string{"1.1.1.1:53", "8.8.8.8:53"}
	lookupPublicAddrs = lookupPublicAddrsDefault
)

func filterPublicHTTP01Names(ctx context.Context, names []string) ([]string, error) {
	names = uniqNames(names)
	if len(names) == 0 {
		return nil, fmt.Errorf("hostname required")
	}
	primary := names[0]
	if err := requirePublicAddress(ctx, primary); err != nil {
		return nil, err
	}
	keep := []string{primary}
	for _, name := range names[1:] {
		if publicNameHasAddress(ctx, name) {
			keep = append(keep, name)
		}
	}
	return keep, nil
}

func requirePublicAddress(ctx context.Context, name string) error {
	if publicNameHasAddress(ctx, name) {
		return nil
	}
	return fmt.Errorf(
		"acme: public DNS cannot resolve %s (timeout or SERVFAIL). "+
			"If this zone is delegated to this host, open UDP/TCP 53 on the "+
			"cloud security group; otherwise publish an A/AAAA at the parent "+
			"DNS provider, then retry",
		name,
	)
}

func publicNameHasAddress(ctx context.Context, name string) bool {
	ips, err := lookupPublicAddrs(ctx, name)
	return err == nil && len(ips) > 0
}

func lookupPublicAddrsDefault(ctx context.Context, name string) ([]net.IPAddr, error) {
	var last error
	for _, server := range publicDNSServers {
		resolver := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				dialer := net.Dialer{Timeout: 3 * time.Second}
				return dialer.DialContext(ctx, network, server)
			},
		}
		lookupCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		ips, err := resolver.LookupIPAddr(lookupCtx, name)
		cancel()
		if err == nil && len(ips) > 0 {
			return ips, nil
		}
		if err != nil {
			last = err
		} else {
			last = fmt.Errorf("no public A/AAAA")
		}
	}
	if last == nil {
		last = fmt.Errorf("no public A/AAAA")
	}
	return nil, last
}
