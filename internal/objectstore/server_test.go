package objectstore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hosting-panel/panel/internal/backup"
)

func TestSignedPutGetList(t *testing.T) {
	dir := t.TempDir()
	srv := httptest.NewServer(&Server{
		Root:      dir,
		AccessKey: "AKIATEST",
		SecretKey: "secret-test",
		Region:    "us-east-1",
	})
	t.Cleanup(srv.Close)
	s := &backup.S3{
		Endpoint:  srv.URL,
		Bucket:    "panel",
		Region:    "us-east-1",
		AccessKey: "AKIATEST",
		SecretKey: "secret-test",
		Client:    srv.Client(),
	}
	if err := s.Put(context.Background(), "acct/one.hpm", []byte("HPM1")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "panel", "acct", "one.hpm")); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(context.Background(), "acct/one.hpm")
	if err != nil || string(got) != "HPM1" {
		t.Fatalf("%q %v", got, err)
	}
	items, err := s.List(context.Background(), "acct")
	if err != nil || len(items) != 1 || items[0].Key != "acct/one.hpm" {
		t.Fatalf("%v %v", items, err)
	}
}

func TestRejectsBadSignature(t *testing.T) {
	dir := t.TempDir()
	srv := httptest.NewServer(&Server{
		Root:      dir,
		AccessKey: "AKIATEST",
		SecretKey: "secret-test",
		Region:    "us-east-1",
	})
	t.Cleanup(srv.Close)
	req, err := http.NewRequest(http.MethodPut, srv.URL+"/panel/x.hpm", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status %d", res.StatusCode)
	}
}
