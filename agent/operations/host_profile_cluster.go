package operations

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	serverProfilePath     = "/etc/panel/server-profile"
	clusterMembershipPath = "/etc/panel/cluster.json"
	clusterSnapshotPath   = "/var/lib/panel/cluster-snapshot.json"
)

func validServerProfile(profile string) bool {
	switch profile {
	case "standard", "mail", "dns":
		return true
	default:
		return false
	}
}

func (h *Host) applyServerProfile(profile string) error {
	if !validServerProfile(profile) {
		return fmt.Errorf("unsupported server profile")
	}
	if _, err := h.ApplyFile(serverProfilePath, []byte(profile+"\n"), 0o644); err != nil {
		return err
	}
	enable, disable := profileServiceUnits(profile)
	if !h.live() {
		return nil
	}
	for _, unit := range enable {
		if err := validateService(unit); err != nil {
			return err
		}
		_, _ = runFixed("/bin/systemctl", "enable", unit)
		_, _ = runFixed("/bin/systemctl", "start", unit)
	}
	for _, unit := range disable {
		if err := validateService(unit); err != nil {
			return err
		}
		if unit == "nginx" || strings.HasPrefix(unit, "panel-") || unit == "postgresql" {
			return fmt.Errorf("refusing to disable management unit %s", unit)
		}
		_, _ = runFixed("/bin/systemctl", "stop", unit)
		_, _ = runFixed("/bin/systemctl", "disable", unit)
	}
	return nil
}

func profileServiceUnits(profile string) (enable, disable []string) {
	web := []string{"php8.3-fpm", "php8.4-fpm", "php8.5-fpm"}
	mail := []string{"postfix", "dovecot", "rspamd"}
	dns := []string{"pdns"}
	switch profile {
	case "standard":
		return append(append(append([]string{}, web...), mail...), dns...), nil
	case "mail":
		return append(append([]string{}, mail...), dns...), web
	case "dns":
		return dns, append(append([]string{}, web...), mail...)
	default:
		return nil, nil
	}
}

type clusterMembership struct {
	Peers []string `json:"peers"`
}

func (h *Host) writeClusterMembership(peers []string) error {
	safe := make([]string, 0, len(peers))
	seen := map[string]bool{}
	for _, peer := range peers {
		peer = strings.TrimSpace(peer)
		if peer == "" || seen[peer] {
			continue
		}
		if !clusterPeerOK(peer) {
			return fmt.Errorf("invalid cluster peer URL")
		}
		seen[peer] = true
		safe = append(safe, peer)
	}
	body, err := json.MarshalIndent(clusterMembership{Peers: safe}, "", "  ")
	if err != nil {
		return err
	}
	_, err = h.ApplyFile(clusterMembershipPath, append(body, '\n'), 0o644)
	return err
}

func clusterPeerOK(raw string) bool {
	if len(raw) > 256 || strings.ContainsAny(raw, " \t\n;|&$`") {
		return false
	}
	if !strings.HasPrefix(raw, "https://") && !strings.HasPrefix(raw, "http://") {
		return false
	}
	rest := strings.TrimPrefix(strings.TrimPrefix(raw, "https://"), "http://")
	host, _, _ := strings.Cut(rest, "/")
	host, _, _ = strings.Cut(host, ":")
	return host != "" && !strings.Contains(host, "..")
}

func (h *Host) writeClusterSnapshot(raw []byte) (Result, error) {
	if len(raw) == 0 || len(raw) > 1<<20 {
		return Result{}, fmt.Errorf("invalid cluster snapshot")
	}
	if !json.Valid(raw) {
		return Result{}, fmt.Errorf("cluster snapshot must be JSON")
	}
	if _, err := h.ApplyFile(clusterSnapshotPath, raw, 0o640); err != nil {
		return Result{}, err
	}
	return Result{OK: true, Message: "cluster snapshot published", ObservedState: "written"}, nil
}

func (h *Host) readClusterSnapshot() ([]byte, error) {
	raw, err := h.readManaged(clusterSnapshotPath, 1<<20)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (h *Host) readServerProfile() string {
	raw, err := h.readManaged(serverProfilePath, 64)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}
