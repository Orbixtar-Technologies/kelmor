package dns

import (
	"strings"
	"testing"

	"github.com/hosting-panel/panel/internal/store"
)

func TestZoneFileAbsoluteMX(t *testing.T) {
	prio := 10
	body := ZoneFile(store.DNSZone{Name: "acme.test"}, []store.DNSRecord{
		{Name: "@", Type: "MX", Content: "mail.acme.test", TTL: 3600, Priority: &prio},
		{Name: "@", Type: "A", Content: "127.0.0.1", TTL: 3600},
	}, 1)
	if !strings.Contains(body, "mail.acme.test.") {
		t.Fatal(body)
	}
	if strings.Contains(body, "127.0.0.1.") {
		t.Fatal(body)
	}
}

func TestZoneFileQuotesTXT(t *testing.T) {
	body := ZoneFile(store.DNSZone{Name: "acme.test"}, []store.DNSRecord{
		{Name: "default._domainkey", Type: "TXT", Content: "v=DKIM1; k=rsa; p=abc", TTL: 3600},
	}, 1)
	if !strings.Contains(body, `"v=DKIM1; k=rsa; p=abc"`) {
		t.Fatal(body)
	}
}
