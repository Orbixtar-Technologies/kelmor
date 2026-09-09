package phases

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ValidationDir is PANEL_VALIDATION_DIR, then /var/lib/panel/validation.
// Repo-local .run/validation is discovered only by LoadValidationEnv.
func ValidationDir() string {
	if v := strings.TrimSpace(os.Getenv("PANEL_VALIDATION_DIR")); v != "" {
		if st, err := os.Stat(v); err == nil && st.IsDir() {
			return v
		}
	}
	if st, err := os.Stat("/var/lib/panel/validation"); err == nil && st.IsDir() {
		return "/var/lib/panel/validation"
	}
	return ""
}

func findRepoValidationDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		cand := filepath.Join(dir, ".run", "validation")
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			return cand
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
	}
}

func loadDotEnv(path string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" || !strings.Contains(line, "=") {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" {
			continue
		}
		out[k] = v
	}
	return out
}

func validationMap() map[string]string {
	dir := ValidationDir()
	if dir == "" {
		return map[string]string{}
	}
	out := map[string]string{}
	for _, name := range []string{"domain.env", "smtp.env", "vm.env"} {
		for k, v := range loadDotEnv(filepath.Join(dir, name)) {
			out[k] = v
		}
	}
	return out
}

// LoadValidationEnv exports validation keys so PublicIPv4, hostname, and
// the mail relay see the same values as scripts/load-validation-env.sh.
func LoadValidationEnv() {
	dir := ValidationDir()
	if dir == "" {
		dir = findRepoValidationDir()
	}
	if dir == "" {
		return
	}
	_ = os.Setenv("PANEL_VALIDATION_DIR", dir)
	m := validationMap()
	for k, v := range m {
		if strings.TrimSpace(os.Getenv(k)) == "" {
			_ = os.Setenv(k, v)
		}
	}
	if os.Getenv("PANEL_PUBLIC_IPV4") == "" && m["VM_PUBLIC_IPV4"] != "" {
		_ = os.Setenv("PANEL_PUBLIC_IPV4", m["VM_PUBLIC_IPV4"])
	}
	if os.Getenv("PANEL_NS1_HOSTNAME") == "" && m["NS1_HOSTNAME"] != "" {
		_ = os.Setenv("PANEL_NS1_HOSTNAME", m["NS1_HOSTNAME"])
	}
	if os.Getenv("PANEL_NS2_HOSTNAME") == "" && m["NS2_HOSTNAME"] != "" {
		_ = os.Setenv("PANEL_NS2_HOSTNAME", m["NS2_HOSTNAME"])
	}
}

// ValidationHostname is TEST_DOMAIN from domain.env when set.
func ValidationHostname() string {
	m := validationMap()
	return strings.TrimSpace(m["TEST_DOMAIN"])
}

func smtpRelaySettings() (host, port, user, pass, tlsMode string, ok bool) {
	m := validationMap()
	host = strings.TrimSpace(m["SMTP_HOST"])
	if host == "" {
		return "", "", "", "", "", false
	}
	port = strings.TrimSpace(m["SMTP_PORT"])
	if port == "" {
		port = "587"
	}
	user = strings.TrimSpace(m["SMTP_USER"])
	tlsMode = strings.TrimSpace(m["SMTP_TLS_MODE"])
	if tlsMode == "" {
		tlsMode = "starttls"
	}
	secret := strings.TrimSpace(m["SMTP_SECRET_PATH"])
	dir := ValidationDir()
	if secret != "" && !filepath.IsAbs(secret) && dir != "" {
		secret = filepath.Join(dir, secret)
	}
	if secret != "" {
		b, err := os.ReadFile(secret)
		if err == nil {
			pass = strings.TrimSpace(string(b))
		}
	}
	if pass == "" {
		pass = strings.TrimSpace(os.Getenv("SMTP_PASSWORD"))
	}
	return host, port, user, pass, tlsMode, true
}

func applySMTPRelay(c Config) error {
	host, port, user, pass, tlsMode, ok := smtpRelaySettings()
	if !ok {
		return nil
	}
	maincf := root(c, "etc/postfix/main.cf")
	relay := fmt.Sprintf("[%s]:%s", host, port)
	tlsLevel := "encrypt"
	if strings.EqualFold(tlsMode, "none") {
		tlsLevel = "may"
	}
	for _, kv := range [][2]string{
		{"relayhost", "relayhost = " + relay + "\n"},
		{"smtp_sasl_auth_enable", "smtp_sasl_auth_enable = yes\n"},
		{"smtp_sasl_password_maps", "smtp_sasl_password_maps = hash:/etc/postfix/sasl_passwd\n"},
		{"smtp_sasl_security_options", "smtp_sasl_security_options = noanonymous\n"},
		{"smtp_tls_security_level", "smtp_tls_security_level = " + tlsLevel + "\n"},
	} {
		if err := replaceConfigLine(maincf, kv[0], kv[1]); err != nil {
			return err
		}
	}
	if user == "" || pass == "" {
		return nil
	}
	saslPath := root(c, "etc/postfix/sasl_passwd")
	body := fmt.Sprintf("%s %s:%s\n", relay, user, pass)
	if err := os.WriteFile(saslPath, []byte(body), 0o600); err != nil {
		return err
	}
	if !c.Dev && installPrefix(c) == "" {
		_ = exec.Command("/usr/sbin/postmap", saslPath).Run()
		_ = exec.Command("/usr/sbin/postfix", "reload").Run()
	}
	return nil
}

func writeValidationPublicLines(pub string) string {
	m := validationMap()
	var b strings.Builder
	b.WriteString("PANEL_PUBLIC_IPV4=" + pub + "\n")
	if v := strings.TrimSpace(m["TEST_DOMAIN"]); v != "" {
		b.WriteString("PANEL_TEST_DOMAIN=" + v + "\n")
	}
	if v := strings.TrimSpace(m["NS1_HOSTNAME"]); v != "" {
		b.WriteString("PANEL_NS1_HOSTNAME=" + v + "\n")
	}
	if v := strings.TrimSpace(m["NS2_HOSTNAME"]); v != "" {
		b.WriteString("PANEL_NS2_HOSTNAME=" + v + "\n")
	}
	return b.String()
}
