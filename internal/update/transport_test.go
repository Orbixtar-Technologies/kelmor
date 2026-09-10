package update

import "testing"

func TestIsLoopbackHost(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "localhost", "::1", "[::1]"} {
		if !isLoopbackHost(host) {
			t.Fatalf("expected loopback for %q", host)
		}
	}
	if isLoopbackHost("203.0.113.10") {
		t.Fatal("public IP must not count as loopback")
	}
}
