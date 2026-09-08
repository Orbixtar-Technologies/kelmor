package mail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSendLimitsAndDecide(t *testing.T) {
	recs := []Recipient{{
		Address: "info@acme.test", Account: "acme42", DailyLimit: 2,
	}}
	body := SendLimits(recs)
	if !strings.Contains(body, "info@acme.test acme42 2") {
		t.Fatal(body)
	}
	dir := t.TempDir()
	limits := filepath.Join(dir, "send-limits")
	if err := os.WriteFile(limits, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	counts := filepath.Join(dir, "counts")
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	for i := 1; i <= 2; i++ {
		act, err := Decide(limits, counts, "info@acme.test", "", now)
		if err != nil || act != "DUNNO" {
			t.Fatalf("msg %d: %s %v", i, act, err)
		}
	}
	act, err := Decide(limits, counts, "INFO@acme.test", "", now)
	if err != nil || !strings.HasPrefix(act, "REJECT") {
		t.Fatalf("over limit: %s %v", act, err)
	}
	act, err = Decide(limits, counts, "probe@localhost", "", now)
	if err != nil || act != "DUNNO" {
		t.Fatalf("unknown sender: %s %v", act, err)
	}
}
