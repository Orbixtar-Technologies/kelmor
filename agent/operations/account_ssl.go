package operations

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/hosting-panel/panel/internal/pkg/validate"
)

func (h *Host) installAccountCertificate(hostname, certPEM, keyPEM, caPEM string) (any, error) {
	host, err := validate.NormalizeDomain(hostname)
	if err != nil {
		return nil, err
	}
	leaf, err := matchingServiceMaterial([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		return nil, err
	}
	if err := leaf.VerifyHostname(host); err != nil {
		return nil, fmt.Errorf("certificate does not cover hostname %s", host)
	}
	fullchain := []byte(strings.TrimSpace(certPEM) + "\n")
	if bundle := strings.TrimSpace(caPEM); bundle != "" {
		if _, err := parseCertificateChain([]byte(bundle)); err != nil {
			return nil, fmt.Errorf("CA bundle PEM is not a readable certificate")
		}
		fullchain = append(fullchain, []byte(bundle+"\n")...)
	}
	if _, err := h.ApplyFile(serviceCertPath(host, ".crt"), fullchain, 0o644); err != nil {
		return nil, err
	}
	if _, err := h.ApplyFile(serviceCertPath(host, ".key"), []byte(keyPEM), 0o600); err != nil {
		return nil, err
	}
	if h.live() {
		_ = reloadNamedService("nginx")
	}
	return map[string]any{
		"ok":                 true,
		"hostname":           host,
		"kind":               "custom",
		"status":             "active",
		"subject":            leaf.Subject.CommonName,
		"issuer":             leaf.Issuer.CommonName,
		"not_after":          leaf.NotAfter.UTC().Format(time.RFC3339),
		"names":              leaf.DNSNames,
		"fingerprint_sha256": sha256Hex(leaf.Raw),
	}, nil
}

func parseCertificateChain(raw []byte) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	rest := raw
	saw := false
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		saw = true
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("certificate PEM is not a readable certificate")
		}
		certs = append(certs, cert)
	}
	if !saw || len(certs) == 0 {
		return nil, fmt.Errorf("certificate PEM is not a readable certificate")
	}
	return certs, nil
}
