package operations

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hosting-panel/panel/internal/pkg/validate"
)

const serviceCertDir = "/var/lib/panel/certs"

type serviceCertSlot struct {
	ID       string
	Label    string
	Kind     string
	CertName string
	Ports    []int
	Reloads  []string
}

func serviceCertSlots(hostname string) []serviceCertSlot {
	slots := []serviceCertSlot{
		{
			ID: "director", Label: "Kelmor Director", Kind: "panel",
			CertName: "panel-portals", Ports: []int{2087}, Reloads: []string{"nginx"},
		},
		{
			ID: "control", Label: "Kelmor Control", Kind: "panel",
			CertName: "panel-portals", Ports: []int{2083}, Reloads: []string{"nginx"},
		},
		{
			ID: "mail", Label: "Mail submission and IMAP", Kind: "mail",
			CertName: "imap.panel.local", Ports: []int{465, 587, 993}, Reloads: []string{"postfix", "dovecot"},
		},
	}
	if hostname != "" {
		slots = append(slots, serviceCertSlot{
			ID: "hostname", Label: "Service hostname", Kind: "hostname",
			CertName: hostname, Ports: []int{2087, 2083}, Reloads: []string{"nginx"},
		})
	}
	return slots
}

func (h *Host) listServiceCertificates(hostname string) (any, error) {
	host := strings.TrimSpace(hostname)
	if host == "" {
		host = h.readPortalHostname()
	}
	items := make([]map[string]any, 0, 4)
	for _, slot := range serviceCertSlots(host) {
		items = append(items, h.serviceCertificateRow(slot, host))
	}
	return map[string]any{
		"items":    items,
		"hostname": host,
		"cert_dir": serviceCertDir,
	}, nil
}

func (h *Host) serviceCertificateRow(slot serviceCertSlot, hostname string) map[string]any {
	row := map[string]any{
		"id":         slot.ID,
		"label":      slot.Label,
		"kind":       slot.Kind,
		"hostname":   serviceSlotHostname(slot, hostname),
		"ports":      slot.Ports,
		"cert_path":  serviceCertPath(slot.CertName, ".crt"),
		"key_path":   serviceCertPath(slot.CertName, ".key"),
		"status":     "missing",
		"reloads":    slot.Reloads,
		"openssl_ok": true,
	}
	raw, err := h.readManaged(serviceCertPath(slot.CertName, ".crt"), 1<<20)
	if err != nil {
		if os.IsNotExist(err) || strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no such file") {
			row["note"] = "No certificate is installed for this service on the host."
			return row
		}
		row["status"] = "unreadable"
		row["openssl_ok"] = false
		row["note"] = "The host certificate file could not be read."
		return row
	}
	leaf := parseCertificatePEM(raw)
	if leaf == nil {
		row["status"] = "unreadable"
		row["openssl_ok"] = false
		row["note"] = "The host file is not a readable PEM certificate."
		return row
	}
	row["status"] = "installed"
	row["subject"] = leaf.Subject.CommonName
	row["issuer"] = leaf.Issuer.CommonName
	row["not_before"] = leaf.NotBefore.UTC()
	row["not_after"] = leaf.NotAfter.UTC()
	row["fingerprint_sha256"] = sha256Hex(leaf.Raw)
	row["names"] = leaf.DNSNames
	if time.Now().After(leaf.NotAfter) {
		row["status"] = "expired"
	}
	if _, err := h.readManaged(serviceCertPath(slot.CertName, ".key"), 1<<20); err != nil {
		row["has_key"] = false
		row["note"] = "Certificate is present but the matching key file is missing."
	} else {
		row["has_key"] = true
	}
	return row
}

func (h *Host) installServiceCertificate(service, hostname, certPEM, keyPEM string) (Result, error) {
	host := strings.TrimSpace(hostname)
	if host == "" {
		host = h.readPortalHostname()
	}
	slot, err := lookupServiceSlot(service, host)
	if err != nil {
		return Result{}, err
	}
	certName := slot.CertName
	if slot.ID == "hostname" {
		normalized, normErr := validate.NormalizeDomain(host)
		if normErr != nil {
			return Result{}, normErr
		}
		certName = normalized
	}
	certBytes := []byte(certPEM)
	keyBytes := []byte(keyPEM)
	leaf, err := matchingServiceMaterial(certBytes, keyBytes)
	if err != nil {
		return Result{}, err
	}
	if _, err := h.ApplyFile(serviceCertPath(certName, ".crt"), certBytes, 0o644); err != nil {
		return Result{}, err
	}
	if _, err := h.ApplyFile(serviceCertPath(certName, ".key"), keyBytes, 0o600); err != nil {
		return Result{}, err
	}
	if h.live() {
		for _, name := range slot.Reloads {
			if err := validateService(name); err != nil {
				return Result{}, err
			}
			_ = reloadNamedService(name)
		}
	}
	return Result{
		OK:            true,
		Message:       "service certificate installed",
		ObservedState: "installed:" + sha256Hex(leaf.Raw),
	}, nil
}

func matchingServiceMaterial(certPEM, keyPEM []byte) (*x509.Certificate, error) {
	if len(certPEM) == 0 || len(keyPEM) == 0 || len(certPEM) > 64<<10 || len(keyPEM) > 64<<10 {
		return nil, fmt.Errorf("certificate and key PEM are required")
	}
	leaf := parseCertificatePEM(certPEM)
	if leaf == nil {
		return nil, fmt.Errorf("certificate PEM is not a readable certificate")
	}
	pub, err := parsePrivateKeyPublic(keyPEM)
	if err != nil {
		return nil, err
	}
	if !publicKeysEqual(leaf.PublicKey, pub) {
		return nil, fmt.Errorf("certificate and key do not match")
	}
	return leaf, nil
}

func parseCertificatePEM(raw []byte) *x509.Certificate {
	rest := raw
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return nil
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		return cert
	}
}

func parsePrivateKeyPublic(raw []byte) (crypto.PublicKey, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("key PEM is not a readable private key")
	}
	switch block.Type {
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("key PEM is not a readable private key")
		}
		return key.Public(), nil
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("key PEM is not a readable private key")
		}
		return key.Public(), nil
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("key PEM is not a readable private key")
		}
		switch typed := key.(type) {
		case *ecdsa.PrivateKey:
			return typed.Public(), nil
		case *rsa.PrivateKey:
			return typed.Public(), nil
		default:
			return nil, fmt.Errorf("unsupported private key type")
		}
	default:
		return nil, fmt.Errorf("key PEM is not a readable private key")
	}
}

func publicKeysEqual(a, b any) bool {
	switch left := a.(type) {
	case *ecdsa.PublicKey:
		right, ok := b.(*ecdsa.PublicKey)
		return ok && left.Equal(right)
	case *rsa.PublicKey:
		right, ok := b.(*rsa.PublicKey)
		return ok && left.Equal(right)
	default:
		return false
	}
}

func lookupServiceSlot(service, hostname string) (serviceCertSlot, error) {
	id := strings.ToLower(strings.TrimSpace(service))
	for _, slot := range serviceCertSlots(hostname) {
		if slot.ID == id {
			return slot, nil
		}
	}
	return serviceCertSlot{}, fmt.Errorf("unknown service certificate target")
}

func serviceCertPath(name, ext string) string {
	return filepath.Join(serviceCertDir, name+ext)
}

func serviceSlotHostname(slot serviceCertSlot, hostname string) string {
	if slot.ID == "hostname" || slot.ID == "director" || slot.ID == "control" {
		if hostname != "" {
			return hostname
		}
	}
	if slot.ID == "mail" {
		return slot.CertName
	}
	return slot.CertName
}

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (h *Host) readPortalHostname() string {
	raw, err := h.readManaged("/var/lib/panel/portal-hostname", 256)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}
