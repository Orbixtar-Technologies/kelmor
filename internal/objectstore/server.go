package objectstore

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hosting-panel/panel/internal/backup"
)

const defaultMaxObjectBytes = 8 << 30

// Server is a path-style S3-compatible store for backup objects.
type Server struct {
	Root      string
	AccessKey string
	SecretKey string
	Region    string
	MaxBytes  int64
	mu        sync.Mutex
}

func (s *Server) maxBytes() int64 {
	if s.MaxBytes > 0 {
		return s.MaxBytes
	}
	return defaultMaxObjectBytes
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bucket, key := splitPath(r.URL.Path)
	if bucket == "" {
		http.Error(w, "bucket required", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPut:
		if key == "" {
			if err := backup.VerifyAWS4(r, nil, s.AccessKey, s.SecretKey, s.Region); err != nil {
				http.Error(w, err.Error(), http.StatusForbidden)
				return
			}
			if err := s.ensureBucket(bucket); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		if err := s.putStream(r, bucket, key); err != nil {
			if strings.Contains(err.Error(), "aws4") || strings.Contains(err.Error(), "authorization") || strings.Contains(err.Error(), "access key") {
				http.Error(w, err.Error(), http.StatusForbidden)
				return
			}
			if strings.Contains(err.Error(), "too large") {
				http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	case http.MethodGet, http.MethodHead:
		if err := backup.VerifyAWS4(r, nil, s.AccessKey, s.SecretKey, s.Region); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		if r.URL.Query().Get("list-type") != "" || key == "" {
			s.list(w, bucket, r.URL.Query().Get("prefix"))
			return
		}
		if err := s.serveObject(w, r, bucket, key); err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
	case http.MethodDelete:
		if err := backup.VerifyAWS4(r, nil, s.AccessKey, s.SecretKey, s.Region); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		if err := s.delete(bucket, key); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) ensureBucket(bucket string) error {
	dir, err := s.safe(bucket, "")
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, 0o750)
}

func (s *Server) putStream(r *http.Request, bucket, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, err := s.safe(bucket, key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	_ = os.Remove(path + ".tmp")
	tmp, err := os.OpenFile(path+".tmp", os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o640)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	limited := io.LimitReader(r.Body, s.maxBytes()+1)
	n, copyErr := io.Copy(io.MultiWriter(tmp, hasher), limited)
	_ = r.Body.Close()
	if copyErr != nil {
		_ = tmp.Close()
		_ = os.Remove(path + ".tmp")
		return copyErr
	}
	if n > s.maxBytes() {
		_ = tmp.Close()
		_ = os.Remove(path + ".tmp")
		return fmt.Errorf("object too large")
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(path + ".tmp")
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(path + ".tmp")
		return err
	}
	digest := hex.EncodeToString(hasher.Sum(nil))
	if err := backup.VerifyAWS4Digest(r, digest, s.AccessKey, s.SecretKey, s.Region); err != nil {
		_ = os.Remove(path + ".tmp")
		return err
	}
	return os.Rename(path+".tmp", path)
}

func (s *Server) serveObject(w http.ResponseWriter, r *http.Request, bucket, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, err := s.safe(bucket, key)
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", st.Size()))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return nil
	}
	_, err = io.Copy(w, f)
	return err
}

func (s *Server) delete(bucket, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, err := s.safe(bucket, key)
	if err != nil {
		return err
	}
	_ = os.Remove(path + ".tmp")
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *Server) list(w http.ResponseWriter, bucket, prefix string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	root, err := s.safe(bucket, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var body strings.Builder
	body.WriteString(`<?xml version="1.0"?><ListBucketResult>`)
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if strings.HasSuffix(path, ".tmp") && time.Since(info.ModTime()) > 24*time.Hour {
			_ = os.Remove(path)
			return nil
		}
		if strings.HasSuffix(path, ".tmp") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		key := filepath.ToSlash(rel)
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			return nil
		}
		_, _ = fmt.Fprintf(&body, "<Contents><Key>%s</Key><Size>%d</Size></Contents>", xmlEscape(key), info.Size())
		return nil
	})
	body.WriteString(`</ListBucketResult>`)
	w.Header().Set("Content-Type", "application/xml")
	_, _ = w.Write([]byte(body.String()))
}

func (s *Server) safe(bucket, key string) (string, error) {
	if strings.Contains(bucket, "..") || strings.Contains(key, "..") {
		return "", fmt.Errorf("invalid object key")
	}
	if bucket == "" || strings.ContainsAny(bucket, `/\`) {
		return "", fmt.Errorf("invalid bucket")
	}
	root := filepath.Clean(s.Root)
	path := filepath.Join(root, bucket)
	if key != "" {
		path = filepath.Join(path, filepath.FromSlash(key))
	}
	clean := filepath.Clean(path)
	if clean != root && !strings.HasPrefix(clean, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes store")
	}
	return clean, nil
}

func splitPath(p string) (bucket, key string) {
	p = strings.Trim(p, "/")
	if p == "" {
		return "", ""
	}
	bucket, rest, _ := strings.Cut(p, "/")
	return bucket, rest
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
