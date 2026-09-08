package objectstore

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hosting-panel/panel/internal/backup"
)

// Server is a path-style S3-compatible store for HPM1 backup objects.
type Server struct {
	Root      string
	AccessKey string
	SecretKey string
	Region    string
	mu        sync.Mutex
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	_ = r.Body.Close()
	if err := backup.VerifyAWS4(r, body, s.AccessKey, s.SecretKey, s.Region); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	bucket, key := splitPath(r.URL.Path)
	if bucket == "" {
		http.Error(w, "bucket required", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPut:
		if key == "" {
			if err := s.ensureBucket(bucket); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		if err := s.put(bucket, key, body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	case http.MethodGet, http.MethodHead:
		if r.URL.Query().Get("list-type") != "" || key == "" {
			s.list(w, bucket, r.URL.Query().Get("prefix"))
			return
		}
		data, err := s.get(bucket, key)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(data)
	case http.MethodDelete:
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

func (s *Server) put(bucket, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, err := s.safe(bucket, key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Server) get(bucket, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, err := s.safe(bucket, key)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func (s *Server) delete(bucket, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, err := s.safe(bucket, key)
	if err != nil {
		return err
	}
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
