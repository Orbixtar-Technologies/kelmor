package operations

import (
	"encoding/json"
	"fmt"
	"strings"
)

const remoteAccessKeyPath = "/etc/panel/remote-access-key"

type RemoteAccessRecord struct {
	Prefix    string `json:"prefix"`
	Hash      string `json:"hash"`
	CreatedAt string `json:"created_at"`
	Revoked   bool   `json:"revoked"`
}

func (h *Host) writeRemoteAccessKey(rec RemoteAccessRecord) (Result, error) {
	rec.Prefix = strings.TrimSpace(rec.Prefix)
	rec.Hash = strings.TrimSpace(rec.Hash)
	if rec.Prefix == "" || rec.Hash == "" {
		return Result{}, fmt.Errorf("remote access key prefix and hash are required")
	}
	if !strings.HasPrefix(rec.Prefix, "hp_remote_") {
		return Result{}, fmt.Errorf("invalid remote access key prefix")
	}
	if len(rec.Hash) != 64 {
		return Result{}, fmt.Errorf("remote access key hash must be sha256 hex")
	}
	for _, r := range rec.Hash {
		hex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
		if !hex {
			return Result{}, fmt.Errorf("remote access key hash must be sha256 hex")
		}
	}
	body, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return Result{}, err
	}
	if _, err := h.ApplyFile(remoteAccessKeyPath, append(body, '\n'), 0o640); err != nil {
		return Result{}, err
	}
	state := "applied"
	if rec.Revoked {
		state = "revoked"
	}
	return Result{OK: true, Message: "remote access key written", ObservedState: state}, nil
}

func (h *Host) readRemoteAccessKey() (any, error) {
	raw, err := h.readManaged(remoteAccessKeyPath, 1<<16)
	if err != nil {
		return nil, err
	}
	var rec RemoteAccessRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil, err
	}
	return rec, nil
}
