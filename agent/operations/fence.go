package operations

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

var ErrStaleFence = errors.New("stale resource fence")

type fenceRecord struct {
	Fence       int64           `json:"fence"`
	OperationID string          `json:"operation_id"`
	Method      string          `json:"method"`
	Fingerprint string          `json:"fingerprint,omitempty"`
	Receipt     json.RawMessage `json:"receipt"`
}

func requestFingerprint(req Request) string {
	sum := sha256.Sum256(append(append([]byte(req.Method), 0), req.Params...))
	return hex.EncodeToString(sum[:])
}

func fenceKey(env Envelope) string {
	if env.ResourceID != "" {
		return env.ResourceID
	}
	return env.OperationID
}

func (h *Host) acceptFence(req Request) (any, bool, error) {
	if req.Env.Fence == 0 {
		return nil, false, nil
	}
	key := fenceKey(req.Env)
	if key == "" {
		return nil, false, ErrStaleFence
	}
	rec := h.loadFence(key)
	if rec.Fence > req.Env.Fence {
		return nil, false, ErrStaleFence
	}
	if rec.Fence == req.Env.Fence && rec.OperationID == req.Env.OperationID && rec.Method == req.Method && rec.Fingerprint == requestFingerprint(req) && rec.Fingerprint != "" && len(rec.Receipt) > 0 {
		var replay any
		_ = json.Unmarshal(rec.Receipt, &replay)
		return replay, true, nil
	}
	if rec.Fence == req.Env.Fence && rec.OperationID != "" && rec.OperationID != req.Env.OperationID {
		return nil, false, ErrStaleFence
	}
	return nil, false, nil
}

func (h *Host) rememberFence(req Request, result any) error {
	if req.Env.Fence == 0 {
		return nil
	}
	key := fenceKey(req.Env)
	if key == "" {
		return nil
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	h.storeFence(key, fenceRecord{
		Fence:       req.Env.Fence,
		OperationID: req.Env.OperationID,
		Method:      req.Method,
		Fingerprint: requestFingerprint(req),
		Receipt:     raw,
	})
	return nil
}

func (h *Host) loadFence(key string) fenceRecord {
	h.ensureFences()
	h.fenceMu.Lock()
	defer h.fenceMu.Unlock()
	if rec, ok := h.fences[key]; ok {
		return rec
	}
	return fenceRecord{}
}

func (h *Host) storeFence(key string, rec fenceRecord) {
	h.ensureFences()
	h.fenceMu.Lock()
	h.fences[key] = rec
	snapshot := make(map[string]fenceRecord, len(h.fences))
	for k, v := range h.fences {
		snapshot[k] = v
	}
	h.fenceMu.Unlock()
	h.persistFences(snapshot)
}

func (h *Host) ensureFences() {
	h.fenceMu.Lock()
	defer h.fenceMu.Unlock()
	if h.fences != nil {
		return
	}
	h.fences = map[string]fenceRecord{}
	path := h.fenceStatePath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var stored map[string]fenceRecord
	if json.Unmarshal(raw, &stored) == nil {
		h.fences = stored
	}
}

func (h *Host) persistFences(snapshot map[string]fenceRecord) {
	path := h.fenceStatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o751); err != nil {
		return
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, raw, 0o640)
}

func (h *Host) fenceStatePath() string {
	if h != nil && h.Root != "" {
		return filepath.Join(h.Root, "var/lib/panel/fences.json")
	}
	if d := os.Getenv("PANEL_STATE_DIR"); d != "" {
		return filepath.Join(d, "fences.json")
	}
	return "/var/lib/panel/fences.json"
}
