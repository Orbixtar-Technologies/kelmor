package backup

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// S3 is an S3-compatible object store (AWS, MinIO). Objects are already HPM1-encrypted.
type S3 struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	Prefix    string
	Client    *http.Client
}

func S3FromEnv() *S3 {
	return &S3{
		Endpoint:  strings.TrimRight(os.Getenv("PANEL_S3_ENDPOINT"), "/"),
		Region:    envDefault("PANEL_S3_REGION", "us-east-1"),
		Bucket:    os.Getenv("PANEL_S3_BUCKET"),
		AccessKey: os.Getenv("PANEL_S3_ACCESS_KEY"),
		SecretKey: os.Getenv("PANEL_S3_SECRET_KEY"),
		Prefix:    strings.Trim(os.Getenv("PANEL_S3_PREFIX"), "/"),
		Client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *S3) objectKey(key string) string {
	key = strings.TrimPrefix(key, "/")
	if s.Prefix == "" {
		return key
	}
	return s.Prefix + "/" + key
}

func (s *S3) url(key string) string {
	return fmt.Sprintf("%s/%s/%s", s.Endpoint, s.Bucket, s.objectKey(key))
}

func (s *S3) Put(ctx context.Context, key string, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, s.url(key), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	s.sign(req, data)
	res, err := s.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("s3 put: %s %s", res.Status, string(b))
	}
	return nil
}

func (s *S3) Get(ctx context.Context, key string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url(key), nil)
	if err != nil {
		return nil, err
	}
	s.sign(req, nil)
	res, err := s.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("s3 get: %s", res.Status)
	}
	return b, nil
}

func (s *S3) Stat(ctx context.Context, key string) (Object, error) {
	b, err := s.Get(ctx, key)
	if err != nil {
		return Object{}, err
	}
	return Object{Key: key, Size: int64(len(b))}, nil
}

func (s *S3) Delete(ctx context.Context, key string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.url(key), nil)
	if err != nil {
		return err
	}
	s.sign(req, nil)
	res, err := s.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 && res.StatusCode != 404 {
		return fmt.Errorf("s3 delete: %s", res.Status)
	}
	return nil
}

func (s *S3) List(ctx context.Context, prefix string) ([]Object, error) {
	return nil, fmt.Errorf("s3 list not implemented")
}

func (s *S3) Verify(ctx context.Context, key, checksum string) error {
	b, err := s.Get(ctx, key)
	if err != nil {
		return err
	}
	if checksum != "" && len(b) == 0 {
		return fmt.Errorf("empty s3 object")
	}
	return nil
}

func (s *S3) client() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return http.DefaultClient
}

func (s *S3) sign(req *http.Request, payload []byte) {
	if s.AccessKey == "" {
		return
	}
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	date := now.Format("20060102")
	payloadHash := sha256Hex(payload)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	canonical := strings.Join([]string{
		req.Method,
		req.URL.EscapedPath(),
		"",
		"host:" + req.URL.Host,
		"x-amz-content-sha256:" + payloadHash,
		"x-amz-date:" + amzDate,
		"",
		"host;x-amz-content-sha256;x-amz-date",
		payloadHash,
	}, "\n")
	scope := date + "/" + s.Region + "/s3/aws4_request"
	sts := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + sha256Hex([]byte(canonical))
	signing := hmacSHA256([]byte("AWS4"+s.SecretKey), []byte(date))
	signing = hmacSHA256(signing, []byte(s.Region))
	signing = hmacSHA256(signing, []byte("s3"))
	signing = hmacSHA256(signing, []byte("aws4_request"))
	sig := hex.EncodeToString(hmacSHA256(signing, []byte(sts)))
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+s.AccessKey+"/"+scope+", SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature="+sig)
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	_, _ = m.Write(data)
	return m.Sum(nil)
}

func envDefault(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

var _ Repository = (*S3)(nil)
