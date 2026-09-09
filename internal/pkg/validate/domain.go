package validate

import (
	"fmt"
	"net"
	"strings"
	"unicode"

	"github.com/hosting-panel/panel/internal/brand"
)

func NormalizeDomain(raw string) (ascii string, err error) {
	s := strings.TrimSpace(strings.ToLower(raw))
	s = strings.TrimSuffix(s, ".")
	if s == "" || len(s) > 253 {
		return "", fmt.Errorf("invalid domain length")
	}
	if strings.ContainsAny(s, " \t\n\r/") {
		return "", fmt.Errorf("invalid domain characters")
	}
	labels := strings.Split(s, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("domain requires at least two labels")
	}
	for _, label := range labels {
		if err := hostnameLabel(label); err != nil {
			return "", err
		}
	}
	return s, nil
}

func LocalPart(s string) error {
	if len(s) < 1 || len(s) > 64 {
		return fmt.Errorf("invalid mailbox local part")
	}
	for i, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if !ok {
			return fmt.Errorf("invalid mailbox local part")
		}
		if i == 0 && (r == '.' || r == '-' || r == '_') {
			return fmt.Errorf("invalid mailbox local part")
		}
	}
	return nil
}

func Username(s string) error {
	if len(s) < 2 || len(s) > 32 {
		return fmt.Errorf("username must be 2-32 characters")
	}
	for i, r := range s {
		if r > unicode.MaxASCII || !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return fmt.Errorf("username must be lowercase alphanumeric")
		}
		if i == 0 && (r == '-' || r == '_' || (r >= '0' && r <= '9')) {
			return fmt.Errorf("username must start with a letter")
		}
	}
	switch s {
	case "root", "panel", "panel-agent", "panel-backup", "www-data", "nobody", "postgres", "mysql":
		return fmt.Errorf("reserved username")
	}
	for _, reserved := range brand.ReservedUsernames {
		if s == reserved {
			return fmt.Errorf("reserved username")
		}
	}
	return nil
}

func hostnameLabel(label string) error {
	if label == "" || len(label) > 63 {
		return fmt.Errorf("invalid DNS label")
	}
	if label[0] == '-' || label[len(label)-1] == '-' {
		return fmt.Errorf("invalid DNS label")
	}
	for _, r := range label {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return fmt.Errorf("invalid DNS label character")
		}
	}
	return nil
}

func IPv4(s string) error {
	ip := net.ParseIP(s)
	if ip == nil || ip.To4() == nil {
		return fmt.Errorf("invalid IPv4 address")
	}
	return nil
}

func CronSchedule(s string) error {
	fields := strings.Fields(s)
	if len(fields) != 5 {
		return fmt.Errorf("cron schedule must have 5 fields")
	}
	for _, f := range fields {
		if f == "" || strings.ContainsAny(f, ";|&$`\n") {
			return fmt.Errorf("invalid cron schedule")
		}
		for _, r := range f {
			ok := (r >= '0' && r <= '9') || r == '*' || r == '/' || r == '-' || r == ','
			if !ok {
				return fmt.Errorf("invalid cron schedule")
			}
		}
	}
	return nil
}

func CronCommand(s string) error {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 256 {
		return fmt.Errorf("invalid cron command")
	}
	if strings.ContainsAny(s, ";|&$`\n") {
		return fmt.Errorf("cron command contains shell metacharacters")
	}
	return nil
}

func IPv6(s string) error {
	ip := net.ParseIP(s)
	if ip == nil || ip.To4() != nil {
		return fmt.Errorf("invalid IPv6 address")
	}
	return nil
}
