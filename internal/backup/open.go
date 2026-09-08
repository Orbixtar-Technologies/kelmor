package backup

import (
	"fmt"
	"os"
	"strings"
)

func Open(destination, localRoot string) (Repository, error) {
	switch strings.ToLower(destination) {
	case "", "local":
		if localRoot == "" {
			localRoot = "/var/lib/panel/backups"
		}
		return &Local{Root: localRoot}, nil
	case "s3":
		s := S3FromEnv()
		if s.Endpoint == "" || s.Bucket == "" {
			return nil, fmt.Errorf("PANEL_S3_ENDPOINT and PANEL_S3_BUCKET required")
		}
		return s, nil
	case "sftp":
		return SFTPFromEnv(), nil
	default:
		return nil, fmt.Errorf("unknown backup destination %q", destination)
	}
}

func DefaultLocalRoot() string {
	if v := os.Getenv("PANEL_BACKUP_ROOT"); v != "" {
		return v
	}
	return "/var/lib/panel/backups"
}
