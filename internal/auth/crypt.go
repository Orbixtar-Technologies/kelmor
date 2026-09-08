package auth

import (
	"fmt"
	"os/exec"
	"strings"
)

// SHA512Crypt returns a $6$ crypt(3) hash for vsftpd pam_pwdfile.
func SHA512Crypt(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password required")
	}
	cmd := exec.Command("/usr/bin/openssl", "passwd", "-6", "-stdin")
	cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/bin", "LC_ALL=C"}
	cmd.Stdin = strings.NewReader(password)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("openssl passwd: %w", err)
	}
	hash := strings.TrimSpace(string(out))
	if !strings.HasPrefix(hash, "$6$") {
		return "", fmt.Errorf("unexpected crypt hash")
	}
	return hash, nil
}
