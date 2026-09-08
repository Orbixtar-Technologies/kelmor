package backup

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

func SignAWS4(req *http.Request, payload []byte, access, secret, region string) {
	if access == "" {
		return
	}
	if region == "" {
		region = "us-east-1"
	}
	now := time.Now().UTC()
	if v := req.Header.Get("X-Amz-Date"); v != "" {
		if t, err := time.Parse("20060102T150405Z", v); err == nil {
			now = t
		}
	}
	amzDate := now.Format("20060102T150405Z")
	date := now.Format("20060102")
	payloadHash := sha256Hex(payload)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	scope := date + "/" + region + "/s3/aws4_request"
	sig := aws4Signature(req, payloadHash, amzDate, secret, scope)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+access+"/"+scope+", SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature="+sig)
}

func VerifyAWS4(req *http.Request, payload []byte, access, secret, region string) error {
	if access == "" {
		return nil
	}
	auth := req.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 ") {
		return fmt.Errorf("missing aws4 authorization")
	}
	parts := map[string]string{}
	for _, field := range strings.Split(strings.TrimPrefix(auth, "AWS4-HMAC-SHA256 "), ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(field), "=")
		if ok {
			parts[k] = v
		}
	}
	cred := parts["Credential"]
	if !strings.HasPrefix(cred, access+"/") {
		return fmt.Errorf("unknown access key")
	}
	scope := strings.TrimPrefix(cred, access+"/")
	if region == "" {
		region = "us-east-1"
	}
	if !strings.Contains(scope, "/"+region+"/s3/") {
		return fmt.Errorf("bad credential scope")
	}
	amzDate := req.Header.Get("X-Amz-Date")
	if amzDate == "" {
		return fmt.Errorf("missing x-amz-date")
	}
	payloadHash := sha256Hex(payload)
	want := aws4Signature(req, payloadHash, amzDate, secret, scope)
	got := parts["Signature"]
	if subtle.ConstantTimeCompare([]byte(want), []byte(got)) != 1 {
		return fmt.Errorf("bad aws4 signature")
	}
	return nil
}

func aws4Signature(req *http.Request, payloadHash, amzDate, secret, scope string) string {
	host := req.Host
	if host == "" && req.URL != nil {
		host = req.URL.Host
	}
	canonical := strings.Join([]string{
		req.Method,
		escapePath(req.URL),
		canonicalQuery(req.URL),
		"host:" + host,
		"x-amz-content-sha256:" + payloadHash,
		"x-amz-date:" + amzDate,
		"",
		"host;x-amz-content-sha256;x-amz-date",
		payloadHash,
	}, "\n")
	sts := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + sha256Hex([]byte(canonical))
	date := strings.SplitN(scope, "/", 2)[0]
	region := "us-east-1"
	if bits := strings.Split(scope, "/"); len(bits) > 1 {
		region = bits[1]
	}
	signing := hmacSHA256([]byte("AWS4"+secret), []byte(date))
	signing = hmacSHA256(signing, []byte(region))
	signing = hmacSHA256(signing, []byte("s3"))
	signing = hmacSHA256(signing, []byte("aws4_request"))
	return hex.EncodeToString(hmacSHA256(signing, []byte(sts)))
}

func escapePath(u *url.URL) string {
	if u == nil {
		return "/"
	}
	if p := u.EscapedPath(); p != "" {
		return p
	}
	return "/"
}

func canonicalQuery(u *url.URL) string {
	if u == nil {
		return ""
	}
	q := u.Query()
	if len(q) == 0 {
		return ""
	}
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		for _, v := range q[k] {
			parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}
	return strings.Join(parts, "&")
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
