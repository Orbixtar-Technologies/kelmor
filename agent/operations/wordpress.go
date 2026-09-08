package operations

import (
	"archive/tar"
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hosting-panel/panel/agent/policy"
	"github.com/hosting-panel/panel/internal/pkg/validate"
)

const officialWordPressURL = "https://wordpress.org/latest.tar.gz"

type WordPressInstall struct {
	Username      string `json:"username"`
	DocumentRoot  string `json:"document_root"`
	DBName        string `json:"db_name"`
	DBUser        string `json:"db_user"`
	DBPassword    string `json:"db_password"`
	DBHost        string `json:"db_host"`
	SiteURL       string `json:"site_url"`
	Title         string `json:"title"`
	AdminUser     string `json:"admin_user"`
	AdminPassword string `json:"admin_password"`
	AdminEmail    string `json:"admin_email"`
}

func (h *Host) installWordPress(p WordPressInstall) (Result, error) {
	if err := validate.Username(p.Username); err != nil {
		return Result{}, err
	}
	if p.DocumentRoot == "" {
		p.DocumentRoot = "/home/" + p.Username + "/public_html"
	}
	doc, err := policy.WithinAccount(p.Username, p.DocumentRoot)
	if err != nil {
		return Result{}, err
	}
	if p.DBName == "" || p.DBUser == "" || p.DBPassword == "" {
		return Result{}, fmt.Errorf("database credentials required")
	}
	if p.DBHost == "" {
		p.DBHost = "127.0.0.1"
	}
	if err := validateWPTitle(p.Title); err != nil {
		return Result{}, err
	}
	if err := validate.Username(p.AdminUser); err != nil {
		return Result{}, fmt.Errorf("admin user: %w", err)
	}
	if len(p.AdminPassword) < 8 {
		return Result{}, fmt.Errorf("admin password too short")
	}
	if p.AdminEmail == "" || !strings.Contains(p.AdminEmail, "@") {
		return Result{}, fmt.Errorf("admin email required")
	}
	realDoc, err := h.resolve(doc)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(realDoc, 0o750); err != nil {
		return Result{}, err
	}
	archive, err := h.wordpressArchive()
	if err != nil {
		return Result{}, err
	}
	stage := "/var/tmp/panel-imports/wordpress-" + p.Username
	if _, err := h.unpackDirectory(archive, stage); err != nil {
		return Result{}, err
	}
	if err := flattenWordPressTree(h, stage, doc); err != nil {
		return Result{}, err
	}
	cfg := wpConfigFile(p)
	if _, err := h.ApplyFile(filepath.Join(doc, "wp-config.php"), []byte(cfg), 0o640); err != nil {
		return Result{}, err
	}
	if h.live() {
		h.hardenWebDocroot(p.Username, doc)
	}
	return Result{OK: true, ObservedState: "installed", Message: "wordpress files written"}, nil
}

func (h *Host) wordpressArchive() (string, error) {
	if env := os.Getenv("PANEL_WORDPRESS_ARCHIVE"); env != "" {
		return env, nil
	}
	cache := "/var/lib/panel/cache/wordpress-latest.tar.gz"
	if rp, err := h.resolve(cache); err == nil {
		if st, err := os.Stat(rp); err == nil && st.Size() > 1000 {
			return cache, nil
		}
	}
	if h.live() {
		if err := h.downloadWordPress(cache); err != nil {
			return "", err
		}
		return cache, nil
	}
	if err := h.writeFixtureWordPress(cache); err != nil {
		return "", err
	}
	return cache, nil
}

func (h *Host) downloadWordPress(dest string) error {
	url := os.Getenv("PANEL_WORDPRESS_URL")
	if url == "" {
		url = officialWordPressURL
	}
	if !strings.HasPrefix(url, "https://wordpress.org/") && !strings.HasPrefix(url, "https://downloads.wordpress.org/") {
		return fmt.Errorf("wordpress download URL not allow-listed")
	}
	if _, err := h.CreateDirectoryTree(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	real, err := h.resolve(dest)
	if err != nil {
		return err
	}
	c := &http.Client{Timeout: 2 * time.Minute}
	res, err := c.Get(url)
	if err != nil {
		return fmt.Errorf("download wordpress: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("download wordpress: %s", res.Status)
	}
	f, err := os.Create(real)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, res.Body)
	_ = f.Close()
	return copyErr
}

func (h *Host) writeFixtureWordPress(dest string) error {
	if _, err := h.CreateDirectoryTree(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	real, err := h.resolve(dest)
	if err != nil {
		return err
	}
	f, err := os.Create(real)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	files := map[string]string{
		"wordpress/index.php":       "<?php echo 'wordpress';\n",
		"wordpress/wp-settings.php": "<?php\n",
		"wordpress/wp-load.php":     "<?php require __DIR__ . '/wp-settings.php';\n",
	}
	for name, body := range files {
		hdr := &tar.Header{Name: name, Mode: 0644, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

func flattenWordPressTree(h *Host, stage, doc string) error {
	realStage, err := h.resolve(stage)
	if err != nil {
		return err
	}
	src := realStage
	if st, err := os.Stat(filepath.Join(realStage, "wordpress")); err == nil && st.IsDir() {
		src = filepath.Join(realStage, "wordpress")
	}
	realDoc, err := h.resolve(doc)
	if err != nil {
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil || rel == "." {
			return err
		}
		if strings.Contains(rel, "..") {
			return fmt.Errorf("wordpress path escape")
		}
		target := filepath.Join(realDoc, rel)
		if !strings.HasPrefix(target, realDoc+string(os.PathSeparator)) {
			return fmt.Errorf("wordpress path escape")
		}
		if info.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		_ = out.Close()
		return copyErr
	})
}

func wpConfigFile(p WordPressInstall) string {
	return "<?php\n" +
		"define('DB_NAME', '" + phpSingle(p.DBName) + "');\n" +
		"define('DB_USER', '" + phpSingle(p.DBUser) + "');\n" +
		"define('DB_PASSWORD', '" + phpSingle(p.DBPassword) + "');\n" +
		"define('DB_HOST', '" + phpSingle(p.DBHost) + "');\n" +
		"define('DB_CHARSET', 'utf8mb4');\n" +
		"define('DB_COLLATE', '');\n" +
		"define('AUTH_KEY', '" + randomSalt() + "');\n" +
		"define('SECURE_AUTH_KEY', '" + randomSalt() + "');\n" +
		"define('LOGGED_IN_KEY', '" + randomSalt() + "');\n" +
		"define('NONCE_KEY', '" + randomSalt() + "');\n" +
		"define('AUTH_SALT', '" + randomSalt() + "');\n" +
		"define('SECURE_AUTH_SALT', '" + randomSalt() + "');\n" +
		"define('LOGGED_IN_SALT', '" + randomSalt() + "');\n" +
		"define('NONCE_SALT', '" + randomSalt() + "');\n" +
		"$table_prefix = 'wp_';\n" +
		"define('WP_DEBUG', false);\n" +
		"if (!defined('ABSPATH')) define('ABSPATH', __DIR__ . '/');\n" +
		"require_once ABSPATH . 'wp-settings.php';\n"
}

func phpSingle(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}

func randomSalt() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "panel-dev-salt"
	}
	return hex.EncodeToString(b)
}

func validateWPTitle(s string) error {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 200 {
		return fmt.Errorf("invalid site title")
	}
	if strings.ContainsAny(s, ";|&$`\n") {
		return fmt.Errorf("invalid site title")
	}
	return nil
}

type unixIDs struct{ uid, gid int }

func lookupUIDGID(username string) (unixIDs, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return unixIDs{}, err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return unixIDs{}, err
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return unixIDs{}, err
	}
	return unixIDs{uid: uid, gid: gid}, nil
}
