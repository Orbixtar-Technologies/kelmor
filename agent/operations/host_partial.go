package operations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	linkedNodesPath      = "/etc/panel/linked-nodes.json"
	externalAuthPath     = "/etc/panel/external-auth.json"
	twoFactorPath        = "/etc/panel/two-factor.json"
	initialQuotaPath     = "/etc/panel/initial-quota"
	quotaUnavailablePath = "/var/lib/panel/quota-unavailable"
)

type linkedNodesFile struct {
	Nodes []string `json:"nodes"`
}

func (h *Host) writeLinkedNodes(nodes []string) error {
	safe := make([]string, 0, len(nodes))
	seen := map[string]bool{}
	for _, node := range nodes {
		node = strings.TrimSpace(node)
		if node == "" || seen[node] {
			continue
		}
		if !clusterPeerOK(node) {
			return fmt.Errorf("invalid linked node URL")
		}
		seen[node] = true
		safe = append(safe, node)
	}
	body, err := json.MarshalIndent(linkedNodesFile{Nodes: safe}, "", "  ")
	if err != nil {
		return err
	}
	if _, err := h.ApplyFile(linkedNodesPath, append(body, '\n'), 0o644); err != nil {
		return err
	}
	merged := append([]string{}, safe...)
	if raw, err := h.readManaged(clusterMembershipPath, 1<<20); err == nil {
		var existing clusterMembership
		if json.Unmarshal(raw, &existing) == nil {
			merged = append(merged, existing.Peers...)
		}
	}
	return h.writeClusterMembership(merged)
}

func (h *Host) writeExternalAuth(spec ExternalAuthSpec) error {
	provider := strings.ToLower(strings.TrimSpace(spec.Provider))
	switch provider {
	case "", "disabled", "local":
		provider = "disabled"
	case "ldap", "oidc":
	default:
		return fmt.Errorf("unsupported external auth provider")
	}
	if spec.Issuer != "" && !strings.HasPrefix(spec.Issuer, "https://") && !strings.HasPrefix(spec.Issuer, "http://") {
		return fmt.Errorf("issuer must be an http(s) URL")
	}
	if spec.LDAPURL != "" && !strings.HasPrefix(spec.LDAPURL, "ldap://") && !strings.HasPrefix(spec.LDAPURL, "ldaps://") {
		return fmt.Errorf("ldap_url must be ldap:// or ldaps://")
	}
	payload := ExternalAuthSpec{
		Provider:        provider,
		Issuer:          strings.TrimSpace(spec.Issuer),
		ClientID:        strings.TrimSpace(spec.ClientID),
		LDAPURL:         strings.TrimSpace(spec.LDAPURL),
		LDAPUserDN:      strings.TrimSpace(spec.LDAPUserDN),
		Enabled:         spec.Enabled && provider != "disabled",
		RequireExternal: spec.RequireExternal && provider != "disabled",
	}
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	_, err = h.ApplyFile(externalAuthPath, append(body, '\n'), 0o640)
	return err
}

func (h *Host) writeTwoFactorPolicy(required bool) error {
	body, err := json.MarshalIndent(map[string]any{"required": required}, "", "  ")
	if err != nil {
		return err
	}
	_, err = h.ApplyFile(twoFactorPath, append(body, '\n'), 0o644)
	return err
}

func (h *Host) writeInitialQuotaPolicy(bytes int64, enforce bool) error {
	if bytes < 0 {
		bytes = 0
	}
	enforceFlag := "off"
	if enforce {
		enforceFlag = "on"
	}
	body := fmt.Sprintf("default_disk_bytes=%d\nenforce=%s\n", bytes, enforceFlag)
	_, err := h.ApplyFile(initialQuotaPath, []byte(body), 0o644)
	return err
}

func (h *Host) probeQuotaStatus() (any, error) {
	unavailable := ""
	if raw, err := h.readManaged(quotaUnavailablePath, 4096); err == nil {
		unavailable = strings.TrimSpace(string(raw))
	}
	setquota := false
	if h.Root == "" {
		if _, err := os.Stat("/usr/sbin/setquota"); err == nil {
			setquota = true
		}
	} else if resolved, err := h.resolve("/usr/sbin/setquota"); err == nil {
		if _, err := os.Stat(resolved); err == nil {
			setquota = true
		}
	}
	homesMounted := false
	if resolved, err := h.resolve("/var/lib/panel/homes"); err == nil {
		if st, err := os.Stat(resolved); err == nil && st.IsDir() {
			homesMounted = true
		}
	}
	policyBytes := int64(0)
	enforce := false
	if raw, err := h.readManaged(initialQuotaPath, 4096); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			switch strings.TrimSpace(key) {
			case "default_disk_bytes":
				n, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
				policyBytes = n
			case "enforce":
				enforce = strings.TrimSpace(value) == "on"
			}
		}
	}
	kernel := unavailable == ""
	return map[string]any{
		"kernel_quota":      kernel,
		"setquota":          setquota || !h.live(),
		"quota_unavailable": unavailable,
		"homes_present":     homesMounted,
		"policy_bytes":      policyBytes,
		"enforce":           enforce,
		"applied":           policyBytes > 0 || enforce,
		"host_path":         initialQuotaPath,
	}, nil
}

func (h *Host) setupInitialQuota(bytes int64, enforce bool) (Result, error) {
	if err := h.writeInitialQuotaPolicy(bytes, enforce); err != nil {
		return Result{}, err
	}
	if h.live() && enforce {
		if _, err := os.Stat("/usr/sbin/setquota"); err != nil {
			_ = h.writeQuotaUnavailable("setquota missing")
		}
	}
	return Result{OK: true, ObservedState: "applied", Message: "initial quota policy written"}, nil
}

func (h *Host) writeQuotaUnavailable(reason string) error {
	rp, err := h.resolve(quotaUnavailablePath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(rp), 0o755); err != nil {
		return err
	}
	return os.WriteFile(rp, []byte(reason+"\n"), 0o644)
}

func (h *Host) verifyLDAPBind(url, userDN, password string) (Result, error) {
	if url == "" || userDN == "" || password == "" {
		return Result{}, fmt.Errorf("ldap url, user dn, and password are required")
	}
	if !strings.HasPrefix(url, "ldap://") && !strings.HasPrefix(url, "ldaps://") {
		return Result{}, fmt.Errorf("ldap_url must be ldap:// or ldaps://")
	}
	if strings.ContainsAny(userDN, "\n\r") || strings.ContainsAny(url, " \t\n;|&$`") {
		return Result{}, fmt.Errorf("ldap bind arguments not permitted")
	}
	if !h.live() {
		note := url + "\t" + userDN + "\n"
		if _, err := h.ApplyFile("/var/lib/panel/ldap-bind-attempts", []byte(note), 0o640); err != nil {
			return Result{}, err
		}
		return Result{OK: true, ObservedState: "sandbox", Message: "ldap bind recorded"}, nil
	}
	out, err := runFixed("/usr/bin/ldapwhoami", "-x", "-H", url, "-D", userDN, "-w", password)
	if err != nil {
		return Result{}, fmt.Errorf("ldap bind failed: %s", strings.TrimSpace(string(out)))
	}
	return Result{OK: true, ObservedState: "bound", Message: strings.TrimSpace(string(out))}, nil
}
