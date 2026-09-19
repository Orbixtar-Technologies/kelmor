package httpserver

import (
	"net/http"
	"time"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/rbac"
)

func (a *API) publishClusterSnapshot(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return
	}
	snapshot := a.clusterSnapshotPayload()
	_, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "WriteClusterSnapshot",
		Params: mustJSON(map[string]any{"snapshot": snapshot}),
	})
	if err != nil {
		a.fail(w, r, 500, "CLUSTER_PUBLISH_ERROR", err.Error(), false)
		return
	}
	a.audit(r, "server.cluster.publish", "server", "", true, nil, map[string]any{
		"packages": len(a.Store.ListPackages()),
	})
	writeJSON(w, 200, map[string]any{"ok": true, "snapshot": snapshot})
}

func (a *API) getClusterSnapshot(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerRead) {
		return
	}
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{Method: "ReadClusterSnapshot"})
	if err == nil && raw != nil {
		writeJSON(w, 200, raw)
		return
	}
	writeJSON(w, 200, a.clusterSnapshotPayload())
}

func (a *API) clusterSnapshotPayload() map[string]any {
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
	return map[string]any{
		"published_at": time.Now().UTC(),
		"note":         "Package and feature-set snapshot for peer Directors. Live multi-node orchestration is not on this Ubuntu Kelmor stack.",
		"packages":     packages,
		"feature_sets": features,
	}
}
