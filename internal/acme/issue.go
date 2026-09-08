package acme

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"

	"golang.org/x/crypto/acme"

	"github.com/hosting-panel/panel/agent/operations"
)

func Issue(ctx context.Context, agent *operations.Host, hostname, contact, directory string) error {
	if directory == "" {
		_, err := agent.Dispatch(ctx, operations.Request{
			Method: "IssueDevCertificate",
			Params: mustJSON(map[string]any{"hostname": hostname, "days": 90}),
		})
		return err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	cl := &acme.Client{Key: key, DirectoryURL: directory}
	acct := &acme.Account{Contact: []string{"mailto:" + contact}}
	if _, err := cl.Register(ctx, acct, acme.AcceptTOS); err != nil && err != acme.ErrAccountAlreadyExists {
		return fmt.Errorf("acme register: %w", err)
	}
	order, err := cl.AuthorizeOrder(ctx, acme.DomainIDs(hostname))
	if err != nil {
		return fmt.Errorf("acme order: %w", err)
	}
	for _, u := range order.AuthzURLs {
		az, err := cl.GetAuthorization(ctx, u)
		if err != nil {
			return err
		}
		var httpCh *acme.Challenge
		for _, ch := range az.Challenges {
			if ch.Type == "http-01" {
				httpCh = ch
				break
			}
		}
		if httpCh == nil {
			return fmt.Errorf("no http-01 challenge for %s", hostname)
		}
		val, err := cl.HTTP01ChallengeResponse(httpCh.Token)
		if err != nil {
			return err
		}
		if _, err := agent.Dispatch(ctx, operations.Request{
			Method: "ApplyACMEChallenge",
			Params: mustJSON(map[string]any{"token": httpCh.Token, "body": val}),
		}); err != nil {
			return err
		}
		if _, err := cl.Accept(ctx, httpCh); err != nil {
			return err
		}
		if _, err := cl.WaitAuthorization(ctx, az.URI); err != nil {
			return err
		}
	}
	csrDER, err := newCSR(hostname, key)
	if err != nil {
		return err
	}
	der, _, err := cl.CreateOrderCert(ctx, order.FinalizeURL, csrDER, true)
	if err != nil {
		return err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der[0]})
	kb, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb})
	if _, err := agent.Dispatch(ctx, operations.Request{
		Method: "ApplyFile",
		Params: mustJSON(map[string]any{"path": "/var/lib/panel/certs/" + hostname + ".crt", "content": string(certPEM), "mode": 0o644}),
	}); err != nil {
		return err
	}
	_, err = agent.Dispatch(ctx, operations.Request{
		Method: "ApplyFile",
		Params: mustJSON(map[string]any{"path": "/var/lib/panel/certs/" + hostname + ".key", "content": string(keyPEM), "mode": 0o600}),
	})
	return err
}

func Directory() string {
	return os.Getenv("PANEL_ACME_DIRECTORY")
}

func newCSR(hostname string, key *ecdsa.PrivateKey) ([]byte, error) {
	tpl := &x509.CertificateRequest{Subject: pkix.Name{CommonName: hostname}, DNSNames: []string{hostname}}
	return x509.CreateCertificateRequest(rand.Reader, tpl, key)
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
