package job

import (
	"fmt"
	"testing"
	"time"

	"github.com/hosting-panel/panel/internal/store"
)

func TestGoneIsPermanent(t *testing.T) {
	if !gone(fmt.Errorf("account missing")) || gone(fmt.Errorf("mariadb timeout")) {
		t.Fatal("gone classification")
	}
}

func TestRetryDelay(t *testing.T) {
	if store.RetryDelay(1) != 0 {
		t.Fatal()
	}
	if store.RetryDelay(5) != 10*time.Minute {
		t.Fatal()
	}
}
