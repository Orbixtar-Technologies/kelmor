package acme

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
)

func TestFilterPublicHTTP01NamesDropsUnresolvedSANs(t *testing.T) {
	orig := lookupPublicAddrs
	t.Cleanup(func() { lookupPublicAddrs = orig })
	lookupPublicAddrs = func(_ context.Context, name string) ([]net.IPAddr, error) {
		switch name {
		case "orbixtar.dpdns.org", "www.orbixtar.dpdns.org":
			return []net.IPAddr{{IP: net.ParseIP("47.85.187.178")}}, nil
		default:
			return nil, errors.New("SERVFAIL no reachable authority")
		}
	}
	got, err := filterPublicHTTP01Names(context.Background(), []string{
		"orbixtar.dpdns.org",
		"www.orbixtar.dpdns.org",
		"mail.orbixtar.dpdns.org",
		"webmail.orbixtar.dpdns.org",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "orbixtar.dpdns.org" ||
		got[1] != "www.orbixtar.dpdns.org" {
		t.Fatalf("filtered names: %v", got)
	}
}

func TestFilterPublicHTTP01NamesRequiresPrimary(t *testing.T) {
	orig := lookupPublicAddrs
	t.Cleanup(func() { lookupPublicAddrs = orig })
	lookupPublicAddrs = func(context.Context, string) ([]net.IPAddr, error) {
		return nil, errors.New("SERVFAIL no reachable authority")
	}
	_, err := filterPublicHTTP01Names(context.Background(), []string{
		"orbixtar.dpdns.org", "mail.orbixtar.dpdns.org",
	})
	if err == nil {
		t.Fatal("expected primary resolution failure")
	}
	msg := err.Error()
	if !strings.Contains(msg, "orbixtar.dpdns.org") ||
		!strings.Contains(msg, "UDP/TCP 53") {
		t.Fatalf("error should name the host and port 53: %v", err)
	}
}
