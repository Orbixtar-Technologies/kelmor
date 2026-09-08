package operations

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
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
	p, err := h.resolve(path)
	if err != nil {
		return Result{}, err
	}
	if mode == 0 {
		mode = 0o640
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return Result{}, err
	}
	tmp := p + ".staging"
	if offset == 0 {
		if err := os.WriteFile(tmp, data, os.FileMode(mode)); err != nil {
			return Result{}, err
		}
	} else {
		st, err := os.Stat(tmp)
		if err != nil {
			return Result{}, err
		}
		if st.Size() != offset {
			return Result{}, fmt.Errorf("file chunk offset mismatch")
		}
		f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_APPEND, os.FileMode(mode))
		if err != nil {
			return Result{}, err
		}
		_, err = f.Write(data)
		_ = f.Close()
		if err != nil {
			return Result{}, err
		}
	}
	_ = os.Chmod(tmp, os.FileMode(mode))
	if !last {
		return Result{OK: true, ObservedState: "chunked"}, nil
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return Result{}, err
	}
	_ = os.Chmod(p, os.FileMode(mode))
	h.chownAccountPath(p)
	return Result{OK: true, ObservedState: "written"}, nil
}
