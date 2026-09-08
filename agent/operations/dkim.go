package operations

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/hosting-panel/panel/internal/pkg/validate"
)

type DKIMRecord struct {
	Domain   string `json:"domain"`
	Selector string `json:"selector"`
	TXT      string `json:"txt"`
}

func (h *Host) ensureDKIM(domain string) (DKIMRecord, error) {
	ascii, err := validate.NormalizeDomain(domain)
	if err != nil {
		return DKIMRecord{}, err
	}
	keyPath := "/var/lib/panel/dkim/" + ascii + "/default.key"
	txtPath := "/var/lib/panel/dkim/" + ascii + "/default.txt"
	priv, err := h.readDKIMPrivate(keyPath)
	if err != nil {
		key, genErr := rsa.GenerateKey(rand.Reader, 2048)
		if genErr != nil {
			return DKIMRecord{}, genErr
		}
		priv = key
		block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}
		if _, err := h.ApplyFile(keyPath, pem.EncodeToMemory(block), 0o640); err != nil {
			return DKIMRecord{}, err
		}
	}
	txt := dkimTXT(&priv.PublicKey)
	if _, err := h.ApplyFile(txtPath, []byte(txt+"\n"), 0o644); err != nil {
		return DKIMRecord{}, err
	}
	if h.live() {
		if abs, err := h.resolve(keyPath); err == nil {
			_ = os.Chmod(abs, 0o640)
			if u, err := user.Lookup("_rspamd"); err == nil {
				uid, _ := strconv.Atoi(u.Uid)
				gid, _ := strconv.Atoi(u.Gid)
				_ = os.Chown(abs, uid, gid)
			}
		}
	}
	return DKIMRecord{Domain: ascii, Selector: "default", TXT: txt}, nil
}

func (h *Host) readDKIMPrivate(path string) (*rsa.PrivateKey, error) {
	abs, err := h.resolve(path)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("dkim key is not PEM")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func dkimTXT(pub *rsa.PublicKey) string {
	der := x509.MarshalPKCS1PublicKey(pub)
	return "v=DKIM1; k=rsa; p=" + base64.StdEncoding.EncodeToString(der)
}

func (h *Host) applyDKIMSigning(domains []string) (Result, error) {
	var b strings.Builder
	b.WriteString("enabled = true;\nallow_username_mismatch = true;\nsign_local = true;\ntry_fallback = false;\n\n")
	seen := map[string]bool{}
	for _, domain := range domains {
		ascii, err := validate.NormalizeDomain(domain)
		if err != nil || seen[ascii] {
			continue
		}
		seen[ascii] = true
		if _, err := h.ensureDKIM(ascii); err != nil {
			return Result{}, err
		}
		fmt.Fprintf(&b, "domain {\n  \"%s\" {\n    path = \"/var/lib/panel/dkim/%s/default.key\";\n    selector = \"default\";\n  }\n}\n", ascii, ascii)
	}
	if _, err := h.ApplyFile("/etc/rspamd/local.d/dkim_signing.conf", []byte(b.String()), 0o644); err != nil {
		return Result{}, err
	}
	if h.live() {
		sighupPidFile("/run/rspamd/rspamd.pid")
	}
	return Result{OK: true, Message: "dkim signing maps written", ObservedState: "applied"}, nil
}

func (h *Host) clearDKIM(domain string) {
	ascii, err := validate.NormalizeDomain(domain)
	if err != nil {
		return
	}
	if dir, err := h.resolve("/var/lib/panel/dkim/" + ascii); err == nil {
		_ = os.RemoveAll(dir)
	}
}
