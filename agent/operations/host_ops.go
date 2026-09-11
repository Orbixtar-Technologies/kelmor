package operations

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const (
	phpMyAdminRootPath = "/usr/share/phpmyadmin"
	webmailRootPath    = "/usr/share/roundcube"
)

type HostProcess struct {
	PID   int    `json:"pid"`
	Name  string `json:"name"`
	User  string `json:"user,omitempty"`
	Cmd   string `json:"command,omitempty"`
	Scope string `json:"scope,omitempty"`
}

type HostApp struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Kind        string `json:"kind"`
	Status      string `json:"status"`
	Path        string `json:"path,omitempty"`
	Description string `json:"description"`
}

type HostRecipe struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Bin         string   `json:"-"`
	Args        []string `json:"-"`
}

func hostRecipes() []HostRecipe {
	return []HostRecipe{
		{ID: "nginx-test", Label: "Test nginx configuration", Description: "Run nginx -t without reloading.", Bin: "/usr/sbin/nginx", Args: []string{"-t"}},
		{ID: "nginx-reload", Label: "Reload nginx", Description: "Reload nginx after a successful config test.", Bin: "/usr/sbin/nginx", Args: []string{"-s", "reload"}},
		{ID: "postfix-queue", Label: "Show mail queue", Description: "List deferred and active Postfix queue entries.", Bin: "/usr/sbin/postqueue", Args: []string{"-p"}},
		{ID: "postfix-flush", Label: "Flush mail queue", Description: "Ask Postfix to retry deferred mail.", Bin: "/usr/sbin/postqueue", Args: []string{"-f"}},
		{ID: "postfix-status", Label: "Postfix status", Description: "Show Postfix service status.", Bin: "/usr/sbin/postfix", Args: []string{"status"}},
	}
}

func (h *Host) listProcesses() ([]HostProcess, error) {
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return []HostProcess{{PID: os.Getpid(), Name: filepath.Base(os.Args[0]), Scope: "serving API instance"}}, nil
	}
	out := make([]HostProcess, 0, 64)
	for _, ent := range ents {
		if !ent.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(ent.Name())
		if err != nil || pid <= 0 {
			continue
		}
		name := readProcFile(filepath.Join("/proc", ent.Name(), "comm"))
		if name == "" {
			continue
		}
		user := ownerName(procUID(ent.Name()))
		cmd := strings.ReplaceAll(readProcFile(filepath.Join("/proc", ent.Name(), "cmdline")), "\x00", " ")
		out = append(out, HostProcess{
			PID:   pid,
			Name:  strings.TrimSpace(name),
			User:  user,
			Cmd:   strings.TrimSpace(cmd),
			Scope: processScope(user, strings.TrimSpace(name)),
		})
		if len(out) >= 300 {
			break
		}
	}
	if len(out) == 0 {
		out = append(out, HostProcess{PID: os.Getpid(), Name: filepath.Base(os.Args[0]), Scope: "serving API instance"})
	}
	return out, nil
}

func readProcFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func procUID(pid string) int {
	body := readProcFile(filepath.Join("/proc", pid, "status"))
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				uid, _ := strconv.Atoi(fields[1])
				return uid
			}
		}
	}
	return -1
}

func processScope(user, name string) string {
	switch name {
	case "panel-api", "panel-dev", "panel-worker", "panel-agent":
		return "control plane"
	case "nginx", "php-fpm", "php-fpm8.3", "php-fpm8.4", "php-fpm8.5":
		return "web"
	case "mysqld", "mariadbd", "postgres":
		return "database"
	case "master", "qmgr", "pickup", "dovecot", "rspamd":
		return "mail"
	}
	if user != "" && user != "root" {
		return "account:" + user
	}
	return "host"
}

func (h *Host) signalProcess(pid int, signal string) (Result, error) {
	if pid <= 1 {
		return Result{}, fmt.Errorf("refusing to signal pid %d", pid)
	}
	var sig syscall.Signal
	switch strings.ToUpper(signal) {
	case "TERM", "SIGTERM":
		sig = syscall.SIGTERM
	case "KILL", "SIGKILL":
		sig = syscall.SIGKILL
	default:
		return Result{}, fmt.Errorf("unsupported signal")
	}
	name := strings.TrimSpace(readProcFile(filepath.Join("/proc", strconv.Itoa(pid), "comm")))
	switch name {
	case "systemd", "init", "panel-agent", "panel-api":
		return Result{}, fmt.Errorf("refusing to signal protected process %s", name)
	}
	if !h.live() {
		return Result{OK: true, Message: fmt.Sprintf("signal %s staged for pid %d", sig, pid), ObservedState: "staged"}, nil
	}
	if err := syscall.Kill(pid, sig); err != nil {
		return Result{}, err
	}
	return Result{OK: true, Message: fmt.Sprintf("sent %s to pid %d", sig, pid), ObservedState: "signaled"}, nil
}

func (h *Host) setRootPassword(password string) (Result, error) {
	if len(password) < 8 || strings.ContainsAny(password, "\n\r:") {
		return Result{}, fmt.Errorf("invalid host password")
	}
	if !h.live() {
		return Result{OK: true, Message: "root password staged", ObservedState: "staged"}, nil
	}
	out, err := runFixedIO("/usr/sbin/chpasswd", []byte("root:"+password+"\n"))
	if err != nil {
		return Result{}, fmt.Errorf("chpasswd: %s", strings.TrimSpace(string(out)))
	}
	return Result{OK: true, Message: "root password set", ObservedState: "set"}, nil
}

func (h *Host) setMariaDBRootPassword(current, next string) (Result, error) {
	if len(next) < 8 || strings.ContainsAny(next, "\n\r;|&$`") {
		return Result{}, fmt.Errorf("invalid database root password")
	}
	if !h.live() {
		return Result{OK: true, Message: "database root password staged", ObservedState: "staged"}, nil
	}
	args := []string{"-u", "root"}
	if current != "" {
		args = append(args, "-p"+current)
	}
	args = append(args, "password", next)
	out, err := runFixed("/usr/bin/mysqladmin", args...)
	if err != nil {
		out2, err2 := runFixed("/usr/bin/mariadb", args...)
		if err2 != nil {
			return Result{}, fmt.Errorf("mysqladmin: %s", strings.TrimSpace(string(out)+" "+string(out2)))
		}
	} else {
		_ = out
	}
	return Result{OK: true, Message: "database root password set", ObservedState: "set"}, nil
}

func (h *Host) listHostApps() []HostApp {
	return []HostApp{
		hostApp("phpmyadmin", "phpMyAdmin", "sql", phpMyAdminRootPath, "SQL browser installed from the host phpmyadmin package and published on phpmyadmin.<domain>."),
		hostApp("roundcube", "Roundcube Webmail", "mail", webmailRootPath, "Webmail installed from the host roundcube packages and published on webmail.<domain>."),
		{ID: "wordpress", Label: "WordPress", Kind: "market", Status: "available", Description: "Install WordPress into an account document root through the existing WordPress API."},
		hostApp("rspamd", "rspamd", "plugin", "/usr/bin/rspamd", "Mail filter already wired as the Postfix milter."),
	}
}

func hostApp(id, label, kind, path, description string) HostApp {
	status := "available"
	if _, err := os.Stat(path); err == nil {
		status = "installed"
	}
	return HostApp{ID: id, Label: label, Kind: kind, Status: status, Path: path, Description: description}
}

func (h *Host) runHostRecipe(id string) (Result, error) {
	var recipe HostRecipe
	for _, entry := range hostRecipes() {
		if entry.ID == id {
			recipe = entry
			break
		}
	}
	if recipe.ID == "" {
		return Result{}, fmt.Errorf("unknown host recipe")
	}
	if !h.live() {
		return Result{OK: true, Message: recipe.Label + " staged", ObservedState: "staged"}, nil
	}
	out, err := runFixed(recipe.Bin, recipe.Args...)
	message := strings.TrimSpace(string(out))
	if err != nil {
		return Result{}, fmt.Errorf("%s", message)
	}
	if message == "" {
		message = recipe.Label + " completed"
	}
	return Result{OK: true, Message: message, ObservedState: "ran"}, nil
}

func (h *Host) listPHPRuntimes() []map[string]any {
	var out []map[string]any
	for _, version := range []string{"8.3", "8.4", "8.5"} {
		bin := "/usr/sbin/php-fpm" + version
		status := "available"
		if _, err := os.Stat(bin); err == nil {
			status = "installed"
		}
		out = append(out, map[string]any{"version": version, "status": status, "binary": bin})
	}
	return out
}

func (h *Host) ensurePHPRuntime(version string) (Result, error) {
	switch version {
	case "8.3", "8.4", "8.5":
	default:
		return Result{}, fmt.Errorf("unsupported PHP version")
	}
	if !h.live() {
		return Result{OK: true, Message: "php-fpm " + version + " staged", ObservedState: "staged"}, nil
	}
	pkg := "php" + version + "-fpm"
	out, err := runFixed("/usr/bin/apt-get", "install", "-y", pkg, "php"+version+"-cli", "php"+version+"-mysql")
	if err != nil {
		return Result{}, fmt.Errorf("apt-get: %s", strings.TrimSpace(string(out)))
	}
	if err := ensureFPM(version); err != nil {
		return Result{}, err
	}
	return Result{OK: true, Message: "php-fpm " + version + " ready", ObservedState: "installed"}, nil
}
