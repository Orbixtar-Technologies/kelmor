package wordpress

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hosting-panel/panel/internal/pkg/validate"
)

var allowed = map[string]bool{
	"core": true, "plugin": true, "theme": true, "db": true, "search-replace": true,
	"maintenance-mode": true, "config": true, "checksum": true,
}

func Command(accountUser, siteDir string, args ...string) *exec.Cmd {
	if err := validate.Username(accountUser); err != nil {
		return nil
	}
	if len(args) == 0 || !allowed[args[0]] {
		return nil
	}
	for _, a := range args {
		if strings.ContainsAny(a, ";|&$`") {
			return nil
		}
	}
	dir := filepath.Clean(siteDir)
	cmd := exec.Command("/usr/bin/sudo", "-u", accountUser, "--", "/usr/local/bin/wp", "--path="+dir)
	cmd.Args = append(cmd.Args, args...)
	cmd.Dir = dir
	cmd.Env = []string{"PATH=/usr/local/bin:/usr/bin"}
	return cmd
}

func DiscoverMarker(docroot string) bool {
	_, err := os.Stat(filepath.Join(filepath.Clean(docroot), "wp-config.php"))
	return err == nil
}

func ValidateArgs(args []string) error {
	if Command("acme42", "/home/acme42/public_html", args...) == nil {
		return fmt.Errorf("wp-cli arguments rejected")
	}
	return nil
}
