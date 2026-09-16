package operations

import (
	"context"
	"encoding/base64"
	"fmt"
)

// MaxRPCLine is the largest JSON line the unix agent protocol accepts.
// File bodies larger than fileChunkSize are sent as ApplyFileChunk frames.
const MaxRPCLine = 1 << 20

const (
	fileChunkSize  = 256 << 10
	maxManagedFile = 2 << 30
)

func (h *Host) applyFileOverSock(path string, content []byte, mode uint32) error {
	if int64(len(content)) > maxManagedFile {
		return fmt.Errorf("file exceeds agent transfer limit")
	}
	if len(content) <= fileChunkSize {
		_, err := CallUnix(context.Background(), h.Sock, Request{
			Method: "ApplyFile",
			Params: mustRaw(map[string]any{
				"path": path, "content_b64": base64.StdEncoding.EncodeToString(content), "mode": mode,
			}),
		})
		return err
	}
	for offset := 0; offset < len(content); {
		end := offset + fileChunkSize
		if end > len(content) {
			end = len(content)
		}
		_, err := h.applyFileChunk(path, content[offset:end], mode, int64(offset), end == len(content))
		if err != nil {
			return err
		}
		offset = end
	}
	return nil
}

func (h *Host) applyFileChunk(path string, data []byte, mode uint32, offset int64, last bool) (Result, error) {
	if offset < 0 || len(data) > fileChunkSize {
		return Result{}, fmt.Errorf("invalid file chunk")
	}
	if offset+int64(len(data)) > maxManagedFile {
		return Result{}, fmt.Errorf("file exceeds agent transfer limit")
	}
	if err := h.rejectHomeWrite(path, offset+int64(len(data))); err != nil {
		return Result{}, err
	}
	if h.Sock != "" {
		_, err := CallUnix(context.Background(), h.Sock, Request{
			Method: "ApplyFileChunk",
			Params: mustRaw(map[string]any{
				"path": path, "content_b64": base64.StdEncoding.EncodeToString(data),
				"mode": mode, "offset": offset, "last": last,
			}),
		})
		if err != nil {
			return Result{}, err
		}
		return Result{OK: true, ObservedState: "chunked"}, nil
	}
	if mode == 0 {
		mode = 0o640
	}
	if err := h.appendManagedChunk(path, data, mode, offset, last); err != nil {
		return Result{}, err
	}
	if last {
		if resolved, err := h.resolve(path); err == nil {
			h.chownAccountPath(resolved)
		}
		return Result{OK: true, ObservedState: "written"}, nil
	}
	return Result{OK: true, ObservedState: "chunked"}, nil
}
