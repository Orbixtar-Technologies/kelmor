package backup

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestS3PutGet(t *testing.T) {
	var mu sync.Mutex
	store := map[string][]byte{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			store[r.URL.Path] = b
			w.WriteHeader(200)
		case http.MethodGet:
			if r.URL.Query().Get("list-type") == "2" {
				pref := r.URL.Query().Get("prefix")
				w.Header().Set("Content-Type", "application/xml")
				_, _ = w.Write([]byte(`<?xml version="1.0"?><ListBucketResult>`))
				for k, v := range store {
					key := strings.TrimPrefix(k, "/panel/")
					if pref != "" && !strings.HasPrefix(key, pref) {
						continue
					}
					_, _ = fmt.Fprintf(w, "<Contents><Key>%s</Key><Size>%d</Size></Contents>", key, len(v))
				}
				_, _ = w.Write([]byte(`</ListBucketResult>`))
				return
			}
			b, ok := store[r.URL.Path]
			if !ok {
				w.WriteHeader(404)
				return
			}
			_, _ = w.Write(b)
		default:
			w.WriteHeader(405)
		}
	}))
	t.Cleanup(srv.Close)
	s := &S3{Endpoint: srv.URL, Bucket: "panel", Region: "us-east-1", Client: srv.Client()}
	if err := s.Put(context.Background(), "acct/one.hpm", []byte("HPM1")); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(context.Background(), "acct/one.hpm")
	if err != nil || string(got) != "HPM1" {
		t.Fatalf("%q %v", got, err)
	}
	items, err := s.List(context.Background(), "acct")
	if err != nil || len(items) != 1 || items[0].Key != "acct/one.hpm" || items[0].Size != 4 {
		t.Fatalf("%v %v", items, err)
	}
}
