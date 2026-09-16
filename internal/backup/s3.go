package backup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	return s.PutStream(ctx, key, bytes.NewReader(data))
}

func (s *S3) Get(ctx context.Context, key string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url(key), nil)
	if err != nil {
		return nil, err
	}
	SignAWS4(req, nil, s.AccessKey, s.SecretKey, s.Region)
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
	SignAWS4(req, nil, s.AccessKey, s.SecretKey, s.Region)
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
	q := url.Values{}
	q.Set("list-type", "2")
	if p := s.objectKey(prefix); p != "" {
		q.Set("prefix", p)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%s?%s", s.Endpoint, s.Bucket, q.Encode()), nil)
	if err != nil {
		return nil, err
	}
	SignAWS4(req, nil, s.AccessKey, s.SecretKey, s.Region)
	res, err := s.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("s3 list: %s %s", res.Status, string(body))
	}
	var parsed struct {
		Contents []struct {
			Key  string `xml:"Key"`
			Size int64  `xml:"Size"`
		} `xml:"Contents"`
	}
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("s3 list xml: %w", err)
	}
	strip := ""
	if s.Prefix != "" {
		strip = strings.TrimSuffix(s.Prefix, "/") + "/"
	}
	var out []Object
	for _, c := range parsed.Contents {
		key := c.Key
		if strip != "" {
			key = strings.TrimPrefix(key, strip)
		}
		out = append(out, Object{Key: key, Size: c.Size})
	}
	return out, nil
}

func (s *S3) Verify(ctx context.Context, key, checksum string) error {
	b, err := s.Get(ctx, key)
	if err != nil {
		return err
	}
	return verifyObject(b, checksum)
}

func (s *S3) PutStream(ctx context.Context, key string, r io.Reader) error {
	tmp, err := os.CreateTemp("", "kelmor-s3-*.part")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		_ = tmp.Close()
		return err
	}
	sum := sha256.New()
	if _, err := io.Copy(sum, tmp); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		_ = tmp.Close()
		return err
	}
	st, err := tmp.Stat()
	if err != nil {
		_ = tmp.Close()
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, s.url(key), tmp)
	if err != nil {
		_ = tmp.Close()
		return err
	}
	req.ContentLength = st.Size()
	req.Header.Set("Content-Type", "application/octet-stream")
	SignAWS4Digest(req, hex.EncodeToString(sum.Sum(nil)), s.AccessKey, s.SecretKey, s.Region)
	res, err := s.client().Do(req)
	if err != nil {
		_ = tmp.Close()
		return err
	}
	defer res.Body.Close()
	_ = tmp.Close()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("s3 put: %s %s", res.Status, string(b))
	}
	return nil
}

func (s *S3) GetStream(ctx context.Context, key string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url(key), nil)
	if err != nil {
		return nil, err
	}
	SignAWS4(req, nil, s.AccessKey, s.SecretKey, s.Region)
	res, err := s.client().Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		_ = res.Body.Close()
		return nil, fmt.Errorf("s3 get: %s", res.Status)
	}
	return res.Body, nil
}

func (s *S3) client() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return http.DefaultClient
}

func envDefault(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

var _ StreamRepository = (*S3)(nil)
