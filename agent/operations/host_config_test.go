package operations

import (
	"strings"
	"testing"
)

func TestNginxDefaultsDoesNotRedeclareGzip(t *testing.T) {
	cases := []HostConfigSpec{
		{Gzip: true},
		{Gzip: false},
	}
	for _, spec := range cases {
		body := nginxDefaults(spec)
		for _, line := range strings.Split(body, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			if strings.HasPrefix(trimmed, "gzip ") || strings.HasPrefix(trimmed, "gzip\t") {
				t.Fatalf("gzip=%v redeclares http gzip (conflicts with nginx.conf): %q", spec.Gzip, line)
			}
		}
		if !strings.Contains(body, "client_max_body_size 64m;") {
			t.Fatalf("gzip=%v missing default client_max_body_size", spec.Gzip)
		}
		if !strings.Contains(body, "keepalive_timeout 65;") {
			t.Fatalf("gzip=%v missing default keepalive_timeout", spec.Gzip)
		}
	}
}
