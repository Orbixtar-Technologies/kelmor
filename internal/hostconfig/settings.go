package hostconfig

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

const FileName = "director-settings.json"

type File struct {
	Values map[string]map[string]string `json:"values"`
}

// PreferredPath is where panel-api (User=panel) can write. The installer
// chowns /var/lib/panel/control but leaves /var/lib/panel itself root:root
// 0755, so a file at the state-dir root is not persistable on a live host.
func PreferredPath(stateDir string) string {
	return filepath.Join(stateDir, "control", FileName)
}

func CandidatePaths(stateDir string) []string {
	return []string{PreferredPath(stateDir), filepath.Join(stateDir, FileName)}
}

func Load(stateDir string) (File, error) {
	out := File{Values: map[string]map[string]string{}}
	if stateDir == "" {
		return out, nil
	}
	for _, path := range CandidatePaths(stateDir) {
		raw, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return out, err
		}
		return parseSettings(raw)
	}
	return out, nil
}

func Store(stateDir string, next File) error {
	if next.Values == nil {
		next.Values = map[string]map[string]string{}
	}
	dir := filepath.Dir(PreferredPath(stateDir))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	tmp := PreferredPath(stateDir) + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, PreferredPath(stateDir))
}

func parseSettings(raw []byte) (File, error) {
	out := File{Values: map[string]map[string]string{}}
	if len(raw) == 0 {
		return out, nil
	}
	var wrap struct {
		Values map[string]map[string]any `json:"values"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return out, err
	}
	for key, fields := range wrap.Values {
		row := map[string]string{}
		for field, value := range fields {
			row[field] = stringifySetting(value)
		}
		out.Values[key] = row
	}
	return out, nil
}

func stringifySetting(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		if typed {
			return "on"
		}
		return "off"
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case json.Number:
		return typed.String()
	default:
		raw, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return string(raw)
	}
}

func Field(f File, key, field, fallback string) string {
	if f.Values == nil {
		return fallback
	}
	row := f.Values[key]
	if row == nil {
		return fallback
	}
	value := strings.TrimSpace(row[field])
	if value == "" {
		return fallback
	}
	return value
}

func BoolField(f File, key, field string, fallback bool) bool {
	raw := strings.ToLower(Field(f, key, field, ""))
	switch raw {
	case "on", "true", "1", "yes":
		return true
	case "off", "false", "0", "no":
		return false
	default:
		return fallback
	}
}

func IntField(f File, key, field string, fallback int) int {
	raw := Field(f, key, field, "")
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}

func Lines(f File, key, field string) []string {
	raw := Field(f, key, field, "")
	if raw == "" {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(strings.ReplaceAll(line, ",", " "))
		if line == "" {
			continue
		}
		for _, part := range strings.Fields(line) {
			part = strings.TrimSpace(part)
			if part == "" || seen[part] {
				continue
			}
			seen[part] = true
			out = append(out, part)
		}
	}
	return out
}

type PasswordPolicy struct {
	MinLength     int
	RequireSymbol bool
}

func PasswordPolicyFrom(f File) PasswordPolicy {
	minLen := IntField(f, "password_strength", "min_length", 0)
	if minLen == 0 {
		minLen = IntField(f, "security_policies", "min_password_length", 12)
	}
	if minLen < 8 {
		minLen = 8
	}
	requireSymbol := BoolField(f, "password_strength", "require_symbol", false)
	return PasswordPolicy{MinLength: minLen, RequireSymbol: requireSymbol}
}

func (p PasswordPolicy) Check(password string) string {
	if len(password) < p.MinLength {
		return "Password must be at least " + strconv.Itoa(p.MinLength) + " characters"
	}
	if p.RequireSymbol {
		ok := false
		for _, r := range password {
			if unicode.IsPunct(r) || unicode.IsSymbol(r) {
				ok = true
				break
			}
		}
		if !ok {
			return "Password must include a symbol"
		}
	}
	return ""
}

func IdleMinutes(f File) int {
	n := IntField(f, "security_policies", "idle_minutes", 0)
	if n < 1 {
		return 0
	}
	if n > 24*60 {
		return 24 * 60
	}
	return n
}

func BruteForce(f File) (enabled bool, maxFailures int, windowMinutes int) {
	enabled = BoolField(f, "brute_force", "enabled", true)
	maxFailures = IntField(f, "brute_force", "max_failures", 8)
	if maxFailures < 1 {
		maxFailures = 1
	}
	windowMinutes = IntField(f, "brute_force", "window_minutes", 1)
	if windowMinutes < 1 {
		windowMinutes = 1
	}
	return enabled, maxFailures, windowMinutes
}

func DemoUsernames(f File) []string {
	return Lines(f, "demo_accounts", "usernames")
}

func IsDemoUsername(f File, username string) bool {
	want := strings.ToLower(strings.TrimSpace(username))
	for _, name := range DemoUsernames(f) {
		if strings.ToLower(name) == want {
			return true
		}
	}
	return false
}

func ZoneTTL(f File) int {
	ttl := IntField(f, "zone_ttl", "ttl", 0)
	if ttl == 0 {
		ttl = IntField(f, "zone_templates", "ttl", 3600)
	}
	if ttl < 60 {
		return 60
	}
	if ttl > 86400 {
		return 86400
	}
	return ttl
}

func DefaultPHP(f File) string {
	ver := Field(f, "tweak_settings", "default_php", "8.3")
	switch ver {
	case "8.2", "8.3", "8.4", "8.5":
		return ver
	default:
		return "8.3"
	}
}

func MaxEmailsHour(f File) int {
	n := IntField(f, "tweak_settings", "max_emails_hour", 0)
	if n < 0 {
		return 0
	}
	return n
}

func BackupUsernames(f File) []string {
	return Lines(f, "backup_users", "usernames")
}

func BackupRetentionDays(f File) int {
	n := IntField(f, "backup_configuration", "retention_days", 14)
	if n < 1 {
		return 1
	}
	return n
}

func BackupDestination(f File) string {
	dest := Field(f, "backup_configuration", "destination", "local")
	switch dest {
	case "local", "sftp", "s3":
		return dest
	default:
		return "local"
	}
}

func ValidIPList(values []string) []string {
	var out []string
	for _, value := range values {
		if parsed := net.ParseIP(value); parsed != nil {
			out = append(out, parsed.String())
			continue
		}
		if _, network, err := net.ParseCIDR(value); err == nil {
			out = append(out, network.String())
		}
	}
	return out
}

var chromeOrDeferredKeys = map[string]bool{
	"theme":             true,
	"locale":            true,
	"customization":     true,
	"remote_access_key": true,
	"mariadb_upgrade":   true,
}

func TwoFactorRequired(f File) bool {
	return BoolField(f, "two_factor", "required", false) ||
		BoolField(f, "security_policies", "require_2fa", false)
}

func ExternalAuthProvider(f File) string {
	provider := strings.ToLower(Field(f, "external_auth", "provider", "disabled"))
	switch provider {
	case "ldap", "oidc", "disabled", "local":
		return provider
	default:
		if Field(f, "external_auth", "ldap_url", "") != "" {
			return "ldap"
		}
		if Field(f, "external_auth", "issuer", "") != "" {
			return "oidc"
		}
		return "disabled"
	}
}

func ExternalAuthRequired(f File) bool {
	return BoolField(f, "external_auth", "require_external", false)
}

func InitialQuotaBytes(f File) int64 {
	mb := IntField(f, "initial_quota", "default_disk_mb", 0)
	if mb < 1 {
		return 0
	}
	return int64(mb) * 1024 * 1024
}

func NeedsHostApply(keys []string) bool {
	for _, key := range keys {
		if !chromeOrDeferredKeys[key] {
			return true
		}
	}
	return false
}

func StateDir(envDir, agentRoot string, sockEmpty bool) string {
	base := strings.TrimSpace(envDir)
	if base == "" {
		base = "/var/lib/panel"
	}
	if sockEmpty && agentRoot != "" {
		return filepath.Join(agentRoot, "var/lib/panel")
	}
	return base
}
