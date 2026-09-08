package operations

import (
	"fmt"
	"os"
	"strings"

	"github.com/hosting-panel/panel/internal/firewall"
)

func (h *Host) applyFirewall() (Result, error) {
	extra := firewall.ExtraListeningTCP()
	body := firewall.Rules(extra)
	path := "/etc/panel/nftables-panel.nft"
	if _, err := h.ApplyFile(path, []byte(body), 0o600); err != nil {
		return Result{}, err
	}
	if !h.live() {
		return Result{OK: true, Message: "firewall table inet panel staged", ObservedState: "staged"}, nil
	}
	resolved, err := h.resolve(path)
	if err != nil {
		return Result{}, err
	}
	if _, err := os.Stat("/usr/sbin/nft"); err != nil {
		return Result{}, fmt.Errorf("nft not installed")
	}
	_, _ = runFixed("/usr/sbin/nft", "delete", "table", "inet", "panel")
	out, err := runFixed("/usr/sbin/nft", "-f", resolved)
	if err != nil {
		return Result{}, fmt.Errorf("nft -f: %s", strings.TrimSpace(string(out)))
	}
	return Result{OK: true, Message: "firewall table inet panel applied", ObservedState: "applied"}, nil
}
