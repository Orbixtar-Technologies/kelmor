package acme

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/acme"

	"github.com/hosting-panel/panel/agent/operations"
)

func Issue(ctx context.Context, agent *operations.Host, hostname, contact, directory string) (time.Time, error) {
	if useDevCertificate(agent, directory) {
		_, err := agent.Dispatch(ctx, operations.Request{
			Method: "IssueDevCertificate",
			Params: mustJSON(map[string]any{"hostname": hostname, "days": 90}),
		})
		if err != nil {
			return time.Time{}, err
		}
		return time.Now().Add(90 * 24 * time.Hour), nil
	}
	acctKey, err := loadOrCreateAccountKey(agent)
	if err != nil {
		return time.Time{}, err
	}
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return time.Time{}, err
	}
	cl := &acme.Client{Key: acctKey, DirectoryURL: directory}
	if insecureDirectory(directory) {
		cl.HTTPClient = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // pebble / lab CAs
			},
		}
	}
	acct := &acme.Account{Contact: []string{"mailto:" + contact}}
	if _, err := cl.Register(ctx, acct, acme.AcceptTOS); err != nil && err != acme.ErrAccountAlreadyExists {
		return time.Time{}, fmt.Errorf("acme register: %w", err)
	}
	order, err := cl.AuthorizeOrder(ctx, acme.DomainIDs(hostname))
	if err != nil {
		return time.Time{}, fmt.Errorf("acme order: %w", err)
	}
	for _, u := range order.AuthzURLs {
		az, err := cl.GetAuthorization(ctx, u)
		if err != nil {
			return time.Time{}, err
		}
		var httpCh *acme.Challenge
		for _, ch := range az.Challenges {
			if ch.Type == "http-01" {
				httpCh = ch
				break
			}
		}
		if httpCh == nil {
			return time.Time{}, fmt.Errorf("no http-01 challenge for %s", hostname)
		}
		val, err := cl.HTTP01ChallengeResponse(httpCh.Token)
		if err != nil {
			return time.Time{}, err
		}
		if _, err := agent.Dispatch(ctx, operations.Request{
			Method: "ApplyACMEChallenge",
			Params: mustJSON(map[string]any{"token": httpCh.Token, "body": val}),
		}); err != nil {
			return time.Time{}, err
		}
		if agent.Root == "" {
			if err := waitHTTP01(hostname, httpCh.Token, val); err != nil {
				return time.Time{}, err
			}
		}
		if _, err := cl.Accept(ctx, httpCh); err != nil {
			return time.Time{}, err
		}
		if _, err := cl.WaitAuthorization(ctx, az.URI); err != nil {
			return time.Time{}, err
		}
	}
	csrDER, err := newCSR(hostname, certKey)
	if err != nil {
		return time.Time{}, err
	}
	der, _, err := cl.CreateOrderCert(ctx, order.FinalizeURL, csrDER, true)
	if err != nil {
		return time.Time{}, err
	}
	var certPEM []byte
	for _, c := range der {
		certPEM = append(certPEM, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c})...)
	}
	kb, err := x509.MarshalECPrivateKey(certKey)
	if err != nil {
		return time.Time{}, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb})
	if _, err := agent.Dispatch(ctx, operations.Request{
		Method: "ApplyFile",
		Params: mustJSON(map[string]any{"path": "/var/lib/panel/certs/" + hostname + ".crt", "content_b64": base64.StdEncoding.EncodeToString(certPEM), "mode": 0o644}),
	}); err != nil {
		return time.Time{}, err
	}
	if _, err := agent.Dispatch(ctx, operations.Request{
		Method: "ApplyFile",
		Params: mustJSON(map[string]any{"path": "/var/lib/panel/certs/" + hostname + ".key", "content_b64": base64.StdEncoding.EncodeToString(keyPEM), "mode": 0o600}),
	}); err != nil {
		return time.Time{}, err
	}
	if _, err := agent.Dispatch(ctx, operations.Request{
		Method: "ReloadService",
		Params: mustJSON(map[string]any{"name": "nginx"}),
	}); err != nil {
		return time.Time{}, err
	}
	exp := time.Now().Add(90 * 24 * time.Hour)
	if leaf := ParseLeafNotAfter(certPEM); leaf != nil {
		exp = leaf.NotAfter
	}
	return exp, nil
}

func waitHTTP01(hostname, token, body string) error {
	url := "http://127.0.0.1/.well-known/acme-challenge/" + token
	deadline := time.Now().Add(8 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		req.Host = hostname
		client := &http.Client{Timeout: 800 * time.Millisecond}
		res, err := client.Do(req)
		if err != nil {
			last = err
			time.Sleep(200 * time.Millisecond)
			continue
		}
		got, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		_ = res.Body.Close()
		if res.StatusCode == 200 && strings.TrimSpace(string(got)) == strings.TrimSpace(body) {
			return nil
		}
		last = fmt.Errorf("http-01 %s returned %d %q", hostname, res.StatusCode, got)
		time.Sleep(200 * time.Millisecond)
	}
	if last == nil {
		last = fmt.Errorf("http-01 challenge not published")
	}
	return last
}

func useDevCertificate(agent *operations.Host, directory string) bool {
	if directory == "" {
		return true
	}
	// Sandboxed agent trees cannot publish HTTP-01 on the live :80 listener.
	return agent != nil && agent.Root != ""
}

func Directory() string {
	if v := os.Getenv("PANEL_ACME_DIRECTORY"); v != "" {
		return v
	}
	b, err := os.ReadFile("/var/lib/panel/acme.directory")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func insecureDirectory(directory string) bool {
	if os.Getenv("PANEL_ACME_INSECURE") == "1" {
		return true
	}
	return strings.Contains(directory, "pebble") || strings.Contains(directory, "127.0.0.1") || strings.Contains(directory, "localhost")
}

func IssuerName(directory string) string {
	if directory == "" {
		return "panel-dev"
	}
	switch {
	case strings.Contains(directory, "staging"):
		return "letsencrypt-staging"
	case strings.Contains(directory, "pebble") || strings.Contains(directory, ":14000"):
		return "pebble"
	default:
		return "letsencrypt"
	}
}

func newCSR(hostname string, key *ecdsa.PrivateKey) ([]byte, error) {
	tpl := &x509.CertificateRequest{Subject: pkix.Name{CommonName: hostname}, DNSNames: []string{hostname}}
	return x509.CreateCertificateRequest(rand.Reader, tpl, key)
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
