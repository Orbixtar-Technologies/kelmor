package httpserver

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/hostconfig"
	"github.com/hosting-panel/panel/internal/rbac"
	"github.com/hosting-panel/panel/internal/store"
)

func (a *API) clusterCapabilities() map[string]any {
	peers := a.clusterPeerURLs()
	return map[string]any{
		"peer_membership":   true,
		"snapshot_publish":  true,
		"snapshot_export":   true,
		"snapshot_import":   true,
		"peer_health_probe": true,
		"live_multi_node":   len(peers) > 0,
		"linked_peers":      len(peers),
	}
}

func (a *API) applyClusterSnapshot(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return
	}
	var in struct {
		Token string `json:"token"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	urls := a.clusterPeerURLs()
	if len(urls) == 0 {
		a.fail(w, r, 400, "VALIDATION", "link at least one peer Director before multi-node apply", false)
		return
	}
	for _, url := range urls {
		if !operations.ClusterPeerOK(url) {
			a.fail(w, r, 400, "VALIDATION", "invalid cluster peer URL", false)
			return
		}
	}
	snapshot := a.clusterSnapshotMap()
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "ApplyClusterSnapshot",
		Params: mustJSON(map[string]any{
			"urls": urls, "snapshot": snapshot, "token": in.Token,
		}),
	})
	if err != nil {
		a.fail(w, r, 500, "CLUSTER_APPLY_ERROR", err.Error(), false)
		return
	}
	job, err := a.enqueueTypedJob(r, &store.Job{
		Type: "cluster.snapshot.apply", ResourceType: "server",
		Payload: map[string]any{"urls": urls, "snapshot": snapshot, "target": "linked-nodes"},
	}, a.auditEvent(r, "", "server.cluster.apply", "server", "", nil, map[string]any{"urls": len(urls)}))
	if err != nil {
		a.fail(w, r, 500, "CLUSTER_APPLY_ERROR", "Could not queue multi-node apply", false)
		return
	}
	items := []any{}
	if payload, ok := raw.(map[string]any); ok {
		if listed, ok := payload["items"].([]map[string]any); ok {
			for _, row := range listed {
				items = append(items, row)
			}
		} else if listed, ok := payload["items"].([]any); ok {
			items = listed
		}
	}
	writeJSON(w, 202, map[string]any{
		"operation_id": job.ID, "items": items, "snapshot": snapshot,
	})
}

func (a *API) publishClusterSnapshot(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return
	}
	snapshot := a.clusterSnapshotPayload()
	job, err := a.enqueueClusterSnapshotJob(r, "cluster.snapshot.publish", "snapshot", snapshot)
	if err != nil {
		a.fail(w, r, 500, "CLUSTER_PUBLISH_ERROR", "Could not queue cluster snapshot publish", false)
		return
	}
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "snapshot": snapshot})
}

func (a *API) importClusterSnapshot(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return
	}
	var in struct {
		Snapshot json.RawMessage `json:"snapshot"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || len(in.Snapshot) == 0 || !json.Valid(in.Snapshot) {
		a.fail(w, r, 400, "VALIDATION", "snapshot must be JSON", false)
		return
	}
	var snapshot map[string]any
	if err := json.Unmarshal(in.Snapshot, &snapshot); err != nil {
		a.fail(w, r, 400, "VALIDATION", "snapshot must be a JSON object", false)
		return
	}
	job, err := a.enqueueClusterSnapshotJob(r, "cluster.snapshot.import", "import", snapshot)
	if err != nil {
		a.fail(w, r, 500, "CLUSTER_IMPORT_ERROR", "Could not queue cluster snapshot import", false)
		return
	}
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "snapshot": snapshot})
}

func (a *API) probeClusterPeers(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return
	}
	var in struct {
		URL string `json:"url"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	urls := []string{}
	if in.URL != "" {
		urls = []string{in.URL}
	} else {
		urls = append(urls, a.clusterPeerURLs()...)
	}
	if len(urls) == 0 {
		a.fail(w, r, 400, "VALIDATION", "peer URL is required", false)
		return
	}
	for _, url := range urls {
		if !operations.ClusterPeerOK(url) {
			a.fail(w, r, 400, "VALIDATION", "invalid cluster peer URL", false)
			return
		}
	}
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "ProbeClusterPeers",
		Params: mustJSON(map[string]any{"urls": urls}),
	})
	if err != nil {
		a.fail(w, r, 500, "CLUSTER_PROBE_ERROR", err.Error(), false)
		return
	}
	job, err := a.enqueueTypedJob(r, &store.Job{
		Type: "cluster.peer.probe", ResourceType: "server",
		Payload: map[string]any{"urls": urls, "target": "peers"},
	}, a.auditEvent(r, "", "server.cluster.probe", "server", "", nil, map[string]any{"urls": len(urls)}))
	if err != nil {
		a.fail(w, r, 500, "CLUSTER_PROBE_ERROR", "Could not queue peer health probe", false)
		return
	}
	items := []any{}
	if payload, ok := raw.(map[string]any); ok {
		if listed, ok := payload["items"].([]map[string]any); ok {
			for _, row := range listed {
				items = append(items, row)
			}
		} else if listed, ok := payload["items"].([]any); ok {
			items = listed
		}
	}
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "items": items})
}

func (a *API) getClusterSnapshot(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerRead) {
		return
	}
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{Method: "ReadClusterSnapshot"})
	if err == nil && raw != nil {
		writeJSON(w, 200, a.attachClusterCapabilities(raw))
		return
	}
	writeJSON(w, 200, a.clusterSnapshotPayload())
}

func (a *API) enqueueClusterSnapshotJob(r *http.Request, jobType, target string, snapshot map[string]any) (*store.Job, error) {
	return a.enqueueTypedJob(r, &store.Job{
		Type: jobType, ResourceType: "server",
		Payload: map[string]any{"snapshot": snapshot, "target": target},
	}, a.auditEvent(r, "", "server."+jobType, "server", "", nil, map[string]any{"target": target}))
}

func (a *API) clusterPeerURLs() []string {
	seen := map[string]bool{}
	out := []string{}
	settings := a.settingsFile()
	for _, source := range [][]string{
		hostconfig.Lines(settings, "configuration_cluster", "peers"),
		hostconfig.Lines(settings, "linked_nodes", "nodes"),
	} {
		for _, url := range source {
			if url == "" || seen[url] {
				continue
			}
			seen[url] = true
			out = append(out, url)
		}
	}
	return out
}

func (a *API) clusterSnapshotPayload() map[string]any {
	return a.attachClusterCapabilities(a.clusterSnapshotMap())
}

func (a *API) clusterSnapshotMap() map[string]any {
	packages := make([]map[string]any, 0)
	for _, pkg := range a.Store.ListPackages() {
		packages = append(packages, map[string]any{
			"id": pkg.ID, "name": pkg.Name, "feature_set_id": pkg.FeatureSetID,
			"disk_bytes": pkg.DiskBytes, "bandwidth_bytes_monthly": pkg.BandwidthBytesMonthly,
			"domains": pkg.Domains, "mailboxes": pkg.Mailboxes, "databases": pkg.Databases,
		})
	}
	features := make([]map[string]any, 0)
	for _, set := range a.Store.ListFeatureSets() {
		features = append(features, map[string]any{
			"id": set.ID, "name": set.Name, "features": set.Features,
		})
	}
	note := "Package and feature-set snapshot for peer Directors. Multi-node apply pushes this snapshot to linked nodes."
	if len(a.clusterPeerURLs()) == 0 {
		note = "Package and feature-set snapshot for this node. Link a peer Director to enable multi-node apply."
	}
	return map[string]any{
		"published_at": time.Now().UTC(),
		"note":         note,
		"packages":     packages,
		"feature_sets": features,
	}
}

func (a *API) attachClusterCapabilities(payload any) map[string]any {
	out := map[string]any{}
	if row, ok := payload.(map[string]any); ok {
		for key, value := range row {
			out[key] = value
		}
	} else if payload != nil {
		out["snapshot"] = payload
	}
	out["capabilities"] = a.clusterCapabilities()
	return out
}
