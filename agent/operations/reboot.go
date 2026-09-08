package operations

import (
	"fmt"
	"os"
	"time"
)

func (h *Host) rebootHost() (Result, error) {
	stamp := time.Now().UTC().Format(time.RFC3339) + "\n"
	if _, err := h.ApplyFile("/var/lib/panel/reboot-requested", []byte(stamp), 0o640); err != nil {
		return Result{}, err
	}
	if !h.live() || os.Getenv("PANEL_ALLOW_REBOOT") != "1" {
		return Result{OK: true, ObservedState: "reboot-scheduled", Message: "reboot recorded; host reboot requires PANEL_ALLOW_REBOOT=1 on a live agent"}, nil
	}
	if out, err := runFixed("/sbin/shutdown", "-r", "now"); err != nil {
		return Result{}, fmt.Errorf("shutdown: %s", string(out))
	}
	return Result{OK: true, ObservedState: "rebooting", Message: "shutdown -r now requested"}, nil
}
