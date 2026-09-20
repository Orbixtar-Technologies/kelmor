package httpserver

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/agent/policy"
	"github.com/hosting-panel/panel/internal/hostconfig"
	"github.com/hosting-panel/panel/internal/rbac"
	"github.com/hosting-panel/panel/internal/store"
)

func (a *API) resetAccountBandwidthUsage(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	if !a.requireAccount(w, r, aid, rbac.AccountsModify) {
		return
	}
	acc := a.Store.GetAccount(aid)
	if acc == nil {
		a.fail(w, r, 404, "NOT_FOUND", "Account not found", false)
		return
	}
	usage := a.Store.GetUsage(acc.ID)
	if usage == nil {
		usage = &store.Usage{AccountID: acc.ID, CollectedAt: time.Now().UTC()}
	}
	usage.BandwidthBytes = 0
	usage.BandwidthHold = false
	usage.CollectedAt = time.Now().UTC()
	a.Store.PutUsage(usage)
	job, err := a.enqueueBandwidthReset(acc, r)
	if err != nil {
		a.fail(w, r, 500, "BANDWIDTH_RESET_ERROR", "Could not queue bandwidth reset", false)
		return
	}
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "account_id": acc.ID})
}

func (a *API) listResellerUsage(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ResellersRead) {
		return
	}
	accounts := a.Store.ListAccounts("", "")
	items := make([]map[string]any, 0)
	for _, reseller := range a.Store.ListResellers() {
		items = append(items, a.resellerUsageRow(&reseller, accounts))
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) resetResellerBandwidth(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.AccountsModify) {
		return
	}
	rid := chi.URLParam(r, "resellerID")
	reseller := a.Store.GetReseller(rid)
	if reseller == nil {
		a.fail(w, r, 404, "NOT_FOUND", "Reseller not found", false)
		return
	}
	var operations []string
	for _, acc := range a.Store.ListAccounts("", "") {
		if acc.ResellerID != rid || !actor(r).CanAccount(acc.ID) {
			continue
		}
		usage := a.Store.GetUsage(acc.ID)
		if usage == nil {
			usage = &store.Usage{AccountID: acc.ID}
		}
		usage.BandwidthBytes = 0
		usage.BandwidthHold = false
		usage.CollectedAt = time.Now().UTC()
		a.Store.PutUsage(usage)
		cp := acc
		job, err := a.enqueueBandwidthReset(&cp, r)
		if err != nil {
			continue
		}
		operations = append(operations, job.ID)
	}
	writeJSON(w, 202, map[string]any{"operations": operations, "cleared": len(operations)})
}

func (a *API) listRestoreInventory(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.BackupsRead) {
		return
	}
	ac := actor(r)
	items := make([]map[string]any, 0)
	for _, backup := range a.Store.ListBackups("") {
		account := a.Store.GetAccount(backup.AccountID)
		if account != nil && !ac.CanAccount(account.ID) {
			continue
		}
		if account == nil && !ac.IsServerScope {
			continue
		}
		username, domain, scope := "", "", "system"
		if account != nil {
			username = account.Username
			domain = account.PrimaryDomain
			scope = "account"
		}
		items = append(items, map[string]any{
			"id":               backup.ID,
			"account_id":       backup.AccountID,
			"account_username": username,
			"primary_domain":   domain,
			"kind":             backup.Kind,
			"state":            backup.State,
			"destination":      backup.Destination,
			"checksum":         backup.Checksum,
			"size_bytes":       backup.SizeBytes,
			"created_at":       backup.CreatedAt,
			"finished_at":      backup.FinishedAt,
			"scope":            scope,
			"restorable":       backupReady(backup),
		})
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) resellerManagerRow(reseller *store.Reseller, accounts []store.Account) map[string]any {
	row := a.resellerUsageRow(reseller, accounts)
	row["user_id"] = reseller.UserID
	row["privilege_mask"] = nonNilStrings(reseller.PrivilegeMask)
	row["nameservers"] = nonNilStrings(reseller.Nameservers)
	names := make([]string, 0)
	seen := map[string]bool{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		names = append(names, name)
	}
	for _, pkg := range a.Store.ListPackages() {
		if pkg.ResellerID == reseller.ID {
			add(pkg.Name)
		}
	}
	for _, acc := range accounts {
		if acc.ResellerID != reseller.ID {
			continue
		}
		if pkg := a.Store.GetPackage(acc.PackageID); pkg != nil {
			add(pkg.Name)
		}
	}
	row["packages"] = names
	row["package_count"] = len(names)
	return row
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func (a *API) resellerUsageRow(reseller *store.Reseller, accounts []store.Account) map[string]any {
	var disk, diskLimit, bandwidth, bandwidthLimit int64
	var total, active, suspended, held int
	for _, acc := range accounts {
		if acc.ResellerID != reseller.ID {
			continue
		}
		total++
		switch acc.Status {
		case "suspended":
			suspended++
		case "active":
			active++
		}
		if usage := a.Store.GetUsage(acc.ID); usage != nil {
			disk += usage.DiskBytes
			bandwidth += usage.BandwidthBytes
			if usage.BandwidthHold {
				held++
			}
		}
		if pkg := a.Store.GetPackage(acc.PackageID); pkg != nil {
			diskLimit += pkg.DiskBytes
			bandwidthLimit += pkg.BandwidthBytesMonthly
		}
	}
	return map[string]any{
		"id":              reseller.ID,
		"name":            reseller.Name,
		"brand_name":      reseller.BrandName,
		"status":          reseller.Status,
		"accounts":        total,
		"active":          active,
		"suspended":       suspended,
		"disk_bytes":      disk,
		"disk_limit":      diskLimit,
		"bandwidth_bytes": bandwidth,
		"bandwidth_limit": bandwidthLimit,
		"bandwidth_holds": held,
	}
}

func (a *API) listSkeleton(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerRead) {
		return
	}
	rel := r.URL.Query().Get("path")
	clean, err := policy.WithinSkeleton(rel)
	if err != nil {
		a.fail(w, r, 400, "PATH_DENIED", err.Error(), false)
		return
	}
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "ListDirectory",
		Params: mustJSON(map[string]any{"path": clean}),
	})
	if err != nil {
		writeJSON(w, 200, map[string]any{"path": rel, "root": "/etc/skel", "items": []any{}})
		return
	}
	body, _ := json.Marshal(raw)
	var listing struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.Unmarshal(body, &listing)
	if listing.Items == nil {
		listing.Items = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"path": rel, "root": "/etc/skel", "absolute": clean, "items": listing.Items})
}

func (a *API) readSkeletonFile(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerRead) {
		return
	}
	clean, err := policy.WithinSkeleton(r.URL.Query().Get("path"))
	if err != nil {
		a.fail(w, r, 400, "PATH_DENIED", err.Error(), false)
		return
	}
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "ReadManagedFile",
		Params: mustJSON(map[string]any{"path": clean}),
	})
	if err != nil {
		a.fail(w, r, 404, "NOT_FOUND", "Skeleton file not found", false)
		return
	}
	result, _ := raw.(operations.Result)
	decoded, err := base64.StdEncoding.DecodeString(result.Message)
	if err != nil {
		decoded = []byte(result.Message)
	}
	writeJSON(w, 200, map[string]any{"path": r.URL.Query().Get("path"), "content": string(decoded)})
}

func (a *API) writeSkeletonFile(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return
	}
	var in struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid skeleton write", false)
		return
	}
	clean, err := policy.WithinSkeleton(in.Path)
	if err != nil || clean == "/etc/skel" {
		a.fail(w, r, 400, "PATH_DENIED", "Choose a file under /etc/skel", false)
		return
	}
	if len(in.Content) > 256<<10 {
		a.fail(w, r, 400, "VALIDATION", "Skeleton file is too large", false)
		return
	}
	if parent := filepath.Dir(clean); parent != "/etc/skel" && parent != "/" {
		if _, err := a.Agent.Dispatch(r.Context(), operations.Request{
			Method: "CreateDirectoryTree",
			Params: mustJSON(map[string]any{"path": parent, "mode": uint32(0o755)}),
		}); err != nil {
			a.fail(w, r, 400, "FILE_ERROR", err.Error(), false)
			return
		}
	}
	_, err = a.Agent.ApplyFile(clean, []byte(in.Content), 0o644)
	if err != nil {
		a.fail(w, r, 400, "FILE_ERROR", err.Error(), false)
		return
	}
	a.audit(r, "server.skeleton.write", "server", "", true, nil, map[string]any{"path": clean})
	writeJSON(w, 200, map[string]any{"ok": true, "path": in.Path})
}

func (a *API) deleteSkeletonFile(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		var in struct {
			Path string `json:"path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		path = in.Path
	}
	clean, err := policy.WithinSkeleton(path)
	if err != nil || clean == "/etc/skel" {
		a.fail(w, r, 400, "PATH_DENIED", "Choose a file under /etc/skel", false)
		return
	}
	_, err = a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "RemoveManagedFile",
		Params: mustJSON(map[string]any{"path": clean}),
	})
	if err != nil {
		a.fail(w, r, 400, "FILE_ERROR", err.Error(), false)
		return
	}
	a.audit(r, "server.skeleton.delete", "server", "", true, nil, map[string]any{"path": clean})
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *API) mkdirSkeleton(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return
	}
	var in struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid mkdir", false)
		return
	}
	clean, err := policy.WithinSkeleton(in.Path)
	if err != nil || clean == "/etc/skel" {
		a.fail(w, r, 400, "PATH_DENIED", "Choose a directory under /etc/skel", false)
		return
	}
	_, err = a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "CreateDirectoryTree",
		Params: mustJSON(map[string]any{"path": clean, "mode": uint32(0o755)}),
	})
	if err != nil {
		a.fail(w, r, 400, "FILE_ERROR", err.Error(), false)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "path": in.Path})
}

func (a *API) getQuotaStatus(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerRead) {
		return
	}
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{Method: "ProbeQuotaStatus"})
	if err != nil {
		a.fail(w, r, 500, "QUOTA_STATUS_ERROR", err.Error(), false)
		return
	}
	writeJSON(w, 200, raw)
}

func (a *API) setupInitialQuota(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return
	}
	var in struct {
		DefaultDiskMB int  `json:"default_disk_mb"`
		Enforce       bool `json:"enforce"`
	}
	in.Enforce = true
	_ = json.NewDecoder(r.Body).Decode(&in)
	if in.DefaultDiskMB < 0 {
		a.fail(w, r, 400, "VALIDATION", "default_disk_mb must be zero or positive", false)
		return
	}
	enforce := "on"
	if !in.Enforce {
		enforce = "off"
	}
	directorSettingsMu.Lock()
	current, err := a.loadDirectorSettings()
	if err != nil {
		directorSettingsMu.Unlock()
		a.fail(w, r, 500, "SETTINGS_READ_ERROR", "Could not read Director settings", false)
		return
	}
	if current.Values == nil {
		current.Values = map[string]map[string]string{}
	}
	current.Values["initial_quota"] = map[string]string{
		"default_disk_mb": strconv.Itoa(in.DefaultDiskMB),
		"enforce":         enforce,
	}
	if err := a.storeDirectorSettings(current); err != nil {
		directorSettingsMu.Unlock()
		a.fail(w, r, 500, "SETTINGS_WRITE_ERROR", err.Error(), false)
		return
	}
	directorSettingsMu.Unlock()
	job, err := a.enqueueTypedJob(r, &store.Job{
		Type: "host.quota.setup", ResourceType: "server",
		Payload: map[string]any{
			"bytes": int64(in.DefaultDiskMB) * 1024 * 1024, "enforce": in.Enforce, "target": "initial-quota",
		},
	}, a.auditEvent(r, "", "host.quota.setup", "server", "", nil, map[string]any{"default_disk_mb": in.DefaultDiskMB}))
	if err != nil {
		a.fail(w, r, 500, "QUOTA_SETUP_ERROR", "Could not queue quota setup", false)
		return
	}
	writeJSON(w, 202, map[string]any{"operation_id": job.ID})
}

func (a *API) listLinkedNodes(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerRead) {
		return
	}
	settings := a.settingsFile()
	linked := hostconfig.Lines(settings, "linked_nodes", "nodes")
	cluster := hostconfig.Lines(settings, "configuration_cluster", "peers")
	hostNodes := []string{}
	if raw, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "ReadManagedFile",
		Params: mustJSON(map[string]any{"path": "/etc/panel/linked-nodes.json"}),
	}); err == nil {
		if result, ok := raw.(operations.Result); ok {
			if decoded, err := base64.StdEncoding.DecodeString(result.Message); err == nil {
				var file struct {
					Nodes []string `json:"nodes"`
				}
				_ = json.Unmarshal(decoded, &file)
				hostNodes = file.Nodes
			}
		}
	}
	seen := map[string]bool{}
	items := make([]map[string]any, 0)
	for _, source := range []struct {
		kind  string
		nodes []string
	}{
		{"linked", linked},
		{"cluster", cluster},
		{"host", hostNodes},
	} {
		for _, node := range source.nodes {
			node = strings.TrimSpace(node)
			if node == "" || seen[node] {
				continue
			}
			seen[node] = true
			items = append(items, map[string]any{"url": node, "source": source.kind})
		}
	}
	writeJSON(w, 200, map[string]any{
		"items": items, "linked_nodes": linked, "cluster_peers": cluster,
		"host_nodes": hostNodes, "host_path": "/etc/panel/linked-nodes.json",
	})
}
