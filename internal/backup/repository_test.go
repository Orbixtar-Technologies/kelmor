package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalVerifyRejectsWrongChecksum(t *testing.T) {
	repo := &Local{Root: t.TempDir()}
	if err := repo.Put(context.Background(), "acct/one.hpm", []byte("not-empty")); err != nil {
		t.Fatal(err)
	}
	if err := repo.Verify(context.Background(), "acct/one.hpm", sha256HexBytes([]byte("different"))); err == nil {
		t.Fatal("accepted wrong checksum")
	}
	sum := sha256.Sum256([]byte("not-empty"))
	if err := repo.Verify(context.Background(), "acct/one.hpm", hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
}

func TestS3VerifyRejectsWrongChecksum(t *testing.T) {
	store := map[string][]byte{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			_ = r.Body.Close()
			store[r.URL.Path] = b
			w.WriteHeader(200)
		case http.MethodGet:
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
	if err := s.Put(context.Background(), "acct/one.hpm", []byte("object")); err != nil {
		t.Fatal(err)
	}
	if err := s.Verify(context.Background(), "acct/one.hpm", sha256HexBytes([]byte("nope"))); err == nil {
		t.Fatal("accepted wrong checksum")
	}
}

func TestSFTPVerifyRejectsWrongChecksum(t *testing.T) {
	s := &SFTP{Root: t.TempDir()}
	if err := s.Put(context.Background(), "acct/one.hpm", []byte("object")); err != nil {
		t.Fatal(err)
	}
	if err := s.Verify(context.Background(), "acct/one.hpm", sha256HexBytes([]byte("nope"))); err == nil {
		t.Fatal("accepted wrong checksum")
	}
	sum := sha256.Sum256([]byte("object"))
	if err := s.Verify(context.Background(), "acct/one.hpm", hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
}

func sha256HexBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
