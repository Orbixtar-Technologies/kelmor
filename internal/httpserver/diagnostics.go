package httpserver

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"time"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/rbac"
)

func (a *API) downloadDiagnostics(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerRead) {
		return
	}
	files := map[string][]byte{}
	if raw, err := a.Agent.Dispatch(r.Context(), operations.Request{Method: "CollectDiagnostics"}); err == nil {
		if bundle, ok := raw.(operations.DiagnosticsBundle); ok {
			for _, file := range bundle.Files {
				files["host/"+file.Name] = []byte(file.Content)
			}
		} else if encoded, encErr := json.Marshal(raw); encErr == nil {
			var decoded operations.DiagnosticsBundle
			if json.Unmarshal(encoded, &decoded) == nil {
				for _, file := range decoded.Files {
					files["host/"+file.Name] = []byte(file.Content)
				}
			}
		}
	}
	settings, _ := a.loadDirectorSettings()
	if body, err := json.MarshalIndent(settings, "", "  "); err == nil {
		files["director/settings.json"] = body
	}
	jobs := a.Store.ListJobs("", 50)
	if body, err := json.MarshalIndent(jobs, "", "  "); err == nil {
		files["director/jobs.json"] = body
	}
	audit := a.Store.ListAudit(50)
	if body, err := json.MarshalIndent(audit, "", "  "); err == nil {
		files["director/audit.json"] = body
	}
	if body, err := json.MarshalIndent(a.Store.ListPackages(), "", "  "); err == nil {
		files["director/packages.json"] = body
	}
	archive, err := tarGzipFiles(files)
	if err != nil {
		a.fail(w, r, 500, "DIAGNOSTICS_ERROR", "Could not build diagnostics archive", false)
		return
	}
	name := "kelmor-diagnostics-" + time.Now().UTC().Format("20060102-150405") + ".tar.gz"
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(archive)
	a.audit(r, "server.diagnostics.download", "server", "", true, nil, map[string]any{"bytes": len(archive)})
}

func tarGzipFiles(files map[string][]byte) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o640,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			_ = tw.Close()
			_ = gz.Close()
			return nil, err
		}
		if _, err := tw.Write(content); err != nil {
			_ = tw.Close()
			_ = gz.Close()
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		_ = gz.Close()
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
