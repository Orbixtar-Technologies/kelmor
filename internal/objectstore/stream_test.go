package objectstore

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hosting-panel/panel/internal/backup"
)

func TestPutStreamsToLimitedTempThenPublishes(t *testing.T) {
	dir := t.TempDir()
	srv := httptest.NewServer(&Server{
		Root:      dir,
		AccessKey: "AKIATEST",
		SecretKey: "secret-test",
		Region:    "us-east-1",
		MaxBytes:  1 << 20,
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
	payload := bytes.Repeat([]byte("S"), 200_000)
	if err := s.PutStream(context.Background(), "acct/stream.hpm", bytes.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "panel", "acct", "stream.hpm")); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetStream(context.Background(), "acct/stream.hpm")
	if err != nil {
		t.Fatal(err)
	}
	defer got.Close()
	body, err := io.ReadAll(got)
	if err != nil || !bytes.Equal(body, payload) {
		t.Fatalf("len %d err %v", len(body), err)
	}
}

func TestPutRejectsOverLimitWithoutPublishing(t *testing.T) {
	dir := t.TempDir()
	srv := httptest.NewServer(&Server{
		Root:      dir,
		AccessKey: "AKIATEST",
		SecretKey: "secret-test",
		Region:    "us-east-1",
		MaxBytes:  64,
	})
	t.Cleanup(srv.Close)
	req, err := http.NewRequest(http.MethodPut, srv.URL+"/panel/acct/big.hpm", bytes.NewReader(bytes.Repeat([]byte("X"), 128)))
	if err != nil {
		t.Fatal(err)
	}
	backup.SignAWS4(req, bytes.Repeat([]byte("X"), 128), "AKIATEST", "secret-test", "us-east-1")
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK {
		t.Fatal("over-limit put succeeded")
	}
	if _, err := os.Stat(filepath.Join(dir, "panel", "acct", "big.hpm")); err == nil {
		t.Fatal("over-limit object published")
	}
}

func TestGetStreamsWithoutBufferingEntireObject(t *testing.T) {
	dir := t.TempDir()
	srv := httptest.NewServer(&Server{
		Root:      dir,
		AccessKey: "AKIATEST",
		SecretKey: "secret-test",
		Region:    "us-east-1",
	})
	t.Cleanup(srv.Close)
	s := &backup.S3{
		Endpoint: srv.URL, Bucket: "panel", Region: "us-east-1",
		AccessKey: "AKIATEST", SecretKey: "secret-test", Client: srv.Client(),
	}
	if err := s.Put(context.Background(), "acct/one.hpm", []byte("stream-me")); err != nil {
		t.Fatal(err)
	}
	rc, err := s.GetStream(context.Background(), "acct/one.hpm")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	var buf strings.Builder
	if _, err := io.Copy(&buf, rc); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "stream-me" {
		t.Fatalf("got %q", buf.String())
	}
}
