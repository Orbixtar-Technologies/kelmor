package store

import (
	"strconv"
	"strings"
	"time"
)

const (
	DefaultInventoryLimit = 50
	MaxInventoryLimit     = 200
	MaxDirectoryEntries   = 500
	MaxBackupList         = 200
	UsageBatchSize        = 25
	CertRenewalBatch      = 50
	MaxWalkInodes         = 10000
	UsageStaleAfter       = 15 * time.Minute
	CertScanInterval      = time.Minute
)

type AccountPage struct {
	Items      []Account
	Usage      map[string]Usage
	NextCursor string
	HasMore    bool
}

type ZonePageItem struct {
	DNSZone
	RecordCount int `json:"record_count"`
}

type ZonePage struct {
	Items      []ZonePageItem
	NextCursor string
	HasMore    bool
}

type AuditPage struct {
	Items      []AuditEvent `json:"items"`
	HasMore    bool         `json:"has_more"`
	NextCursor string       `json:"next_cursor,omitempty"`
}

func ClampPageLimit(limit, fallback int) int {
	if limit <= 0 {
		limit = fallback
	}
	if limit > MaxInventoryLimit {
		return MaxInventoryLimit
	}
	return limit
}

func EncodeAuditCursor(event AuditEvent) string {
	if event.ID == "" || event.OccurredAt.IsZero() {
		return ""
	}
	return event.OccurredAt.UTC().Format(time.RFC3339Nano) + "|" + event.ID
}

func DecodeAuditCursor(cursor string) (time.Time, string, bool) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return time.Time{}, "", false
	}
	stamp, id, ok := strings.Cut(cursor, "|")
	if !ok || id == "" {
		return time.Time{}, "", false
	}
	parsed, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, stamp)
	}
	if err != nil {
		return time.Time{}, "", false
	}
	return parsed, id, true
}

func AfterUsername(cursor, username string) bool {
	return cursor == "" || username > cursor
}

func AfterName(cursor, name string) bool {
	return cursor == "" || name > cursor
}

func ParseLimit(raw string, fallback int) int {
	if raw == "" {
		return ClampPageLimit(fallback, fallback)
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return ClampPageLimit(fallback, fallback)
	}
	return ClampPageLimit(n, fallback)
}
