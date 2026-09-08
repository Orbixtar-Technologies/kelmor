package job

import (
	"testing"
	"time"

	"github.com/hosting-panel/panel/internal/store"
)

func TestRetryDelay(t *testing.T) {
	if store.RetryDelay(1) != 0 {
		t.Fatal()
	}
	if store.RetryDelay(5) != 10*time.Minute {
		t.Fatal()
	}
}
