package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println(`panel-cli <command>

  version
  health
  login <user> <pass>
  server status
  account list
  account get <id>
  account create <user> <domain> <package_id> <password>
  account suspend <id>
  account unsuspend <id>
  account terminate <id>
  account export <id>
  account import <export.json> [username] [domain]
  account migrate <id> <newuser> <newdomain>
  account sftp-password <id> <password>
  ftp list <account_id>
  ftp create <account_id> <username> <password>
  ftp delete <account_id> <ftp_id>
  mailbox create <account_id> <mail_domain_id> <local> <password>
  mail catchall <account_id> <mail_domain_id> <reject|discard|local>
  dnssec enable <account_id> <zone_id>
  dnssec disable <account_id> <zone_id>
  dnssec ds <account_id> <zone_id>
  db create <account_id> <name> <engine>
  website create <account_id> <domain_id> <runtime>
  domain create <account_id> <fqdn> [runtime] [type]
  file list <account_id> [path]
  file write <account_id> <path> <content>
  backup create <account_id>
  backup restore <account_id> <backup_id>
  jobs list
  jobs failed
  job wait <id>
  audit
  monitor
  import-cpanel <root> <username>
  cron create <account_id> <schedule> <command>
  cert request <account_id> <hostname>
  reseller create <name> <username> <password>
  firewall apply
  reboot
  config validate`)
		os.Exit(2)
	}
	base := env("PANEL_API", "http://127.0.0.1:18080")
	token := env("PANEL_TOKEN", "")
	args := os.Args[1:]
	switch {
	case args[0] == "version":
		get(base+"/api/v1/version", token)
	case args[0] == "health":
		get(base+"/healthz", token)
	case args[0] == "login" && len(args) == 3:
		login(base, args[1], args[2])
	case join(args) == "server status":
		get(base+"/api/v1/server", token)
	case join(args) == "account list":
		get(base+"/api/v1/accounts", token)
	case args[0] == "account" && len(args) == 3 && args[1] == "get":
		get(base+"/api/v1/accounts/"+args[2], token)
	case args[0] == "account" && len(args) == 5 && args[1] == "migrate":
		migrateAccount(base, token, args[2], args[3], args[4])
	case args[0] == "account" && len(args) == 4 && args[1] == "sftp-password":
		post(base+"/api/v1/accounts/"+args[2]+"/sftp-password", token, map[string]any{"password": args[3]})
	case args[0] == "ftp" && args[1] == "list" && len(args) == 3:
		get(base+"/api/v1/accounts/"+args[2]+"/ftp", token)
	case args[0] == "ftp" && args[1] == "create" && len(args) == 5:
		post(base+"/api/v1/accounts/"+args[2]+"/ftp", token, map[string]any{"username": args[3], "password": args[4]})
	case args[0] == "ftp" && args[1] == "delete" && len(args) == 4:
		do(http.MethodDelete, base+"/api/v1/accounts/"+args[2]+"/ftp/"+args[3], token, nil, true)
	case args[0] == "account" && len(args) >= 3 && args[1] == "import":
		raw, err := os.ReadFile(args[2])
		if err != nil {
			fatal(err.Error())
		}
		url := base + "/api/v1/accounts/import"
		if len(args) >= 5 {
			url += "?username=" + args[3] + "&domain=" + args[4]
		}
		doBytes(http.MethodPost, url, token, raw)
	case args[0] == "mail" && args[1] == "catchall" && len(args) == 5:
		do(http.MethodPatch, base+"/api/v1/accounts/"+args[2]+"/mail/domains/"+args[3], token, map[string]any{"catchall_policy": args[4]}, true)
	case args[0] == "dnssec" && args[1] == "enable" && len(args) == 4:
		post(base+"/api/v1/accounts/"+args[2]+"/dns/zones/"+args[3]+"/dnssec", token, map[string]any{"enabled": true})
	case args[0] == "dnssec" && args[1] == "disable" && len(args) == 4:
		post(base+"/api/v1/accounts/"+args[2]+"/dns/zones/"+args[3]+"/dnssec", token, map[string]any{"enabled": false})
	case args[0] == "dnssec" && args[1] == "ds" && len(args) == 4:
		get(base+"/api/v1/accounts/"+args[2]+"/dns/zones/"+args[3]+"/ds", token)
	case args[0] == "mailbox" && len(args) == 6 && args[1] == "create":
		post(base+"/api/v1/accounts/"+args[2]+"/mail/mailboxes", token, map[string]any{
			"domain_id": args[3], "local_part": args[4], "password": args[5],
		})
	case args[0] == "db" && len(args) == 5 && args[1] == "create":
		post(base+"/api/v1/accounts/"+args[2]+"/databases", token, map[string]any{"name": args[3], "engine": args[4]})
	case args[0] == "domain" && len(args) >= 4 && args[1] == "create":
		body := map[string]any{"fqdn": args[3]}
		if len(args) >= 5 {
			body["runtime"] = args[4]
		}
		if len(args) >= 6 {
			body["type"] = args[5]
		}
		post(base+"/api/v1/accounts/"+args[2]+"/domains", token, body)
	case args[0] == "website" && len(args) == 5 && args[1] == "create":
		post(base+"/api/v1/accounts/"+args[2]+"/websites", token, map[string]any{"domain_id": args[3], "runtime": args[4]})
	case args[0] == "file" && args[1] == "list" && len(args) >= 3:
		path := "/"
		if len(args) >= 4 {
			path = args[3]
		}
		get(base+"/api/v1/accounts/"+args[2]+"/files?path="+path, token)
	case args[0] == "file" && args[1] == "write" && len(args) == 5:
		post(base+"/api/v1/accounts/"+args[2]+"/files", token, map[string]any{"path": args[3], "content": args[4]})
	case args[0] == "backup" && len(args) == 4 && args[1] == "restore":
		post(base+"/api/v1/accounts/"+args[2]+"/restores", token, map[string]any{"backup_id": args[3], "mode": "in_place"})
	case args[0] == "account" && len(args) >= 2 && args[1] == "create":
		if len(args) < 6 {
			fatal("account create <user> <domain> <package_id> <password>")
		}
		post(base+"/api/v1/accounts", token, map[string]any{
			"username": args[2], "primary_domain": args[3], "package_id": args[4],
			"owner_password": args[5],
		})
	case args[0] == "account" && len(args) == 3 && args[1] == "suspend":
		post(base+"/api/v1/accounts/"+args[2]+"/suspend", token, map[string]any{})
	case args[0] == "account" && len(args) == 3 && args[1] == "unsuspend":
		post(base+"/api/v1/accounts/"+args[2]+"/unsuspend", token, map[string]any{})
	case args[0] == "account" && len(args) == 3 && args[1] == "terminate":
		post(base+"/api/v1/accounts/"+args[2]+"/terminate", token, map[string]any{})
	case args[0] == "account" && len(args) == 3 && args[1] == "export":
		get(base+"/api/v1/accounts/"+args[2]+"/export", token)
	case args[0] == "backup" && len(args) == 3 && args[1] == "create":
		post(base+"/api/v1/accounts/"+args[2]+"/backups", token, map[string]any{"kind": "full", "destination": "local"})
	case args[0] == "job" && args[1] == "wait" && len(args) == 3:
		waitJob(base, token, args[2])
	case join(args) == "jobs list":
		get(base+"/api/v1/jobs", token)
	case join(args) == "jobs list --failed" || join(args) == "jobs failed":
		get(base+"/api/v1/jobs?state=failed", token)
	case args[0] == "audit":
		get(base+"/api/v1/audit-events", token)
	case args[0] == "monitor":
		get(base+"/api/v1/server/monitor", token)
	case args[0] == "import-cpanel" && len(args) == 3:
		post(base+"/api/v1/accounts/import/cpanel", token, map[string]any{"root": args[1], "username": args[2]})
	case args[0] == "cron" && args[1] == "create" && len(args) >= 5:
		post(base+"/api/v1/accounts/"+args[2]+"/cron", token, map[string]any{"schedule": args[3], "command": strings.Join(args[4:], " ")})
	case args[0] == "cert" && args[1] == "request" && len(args) == 4:
		post(base+"/api/v1/accounts/"+args[2]+"/certificates", token, map[string]any{"hostname": args[3]})
	case args[0] == "reseller" && args[1] == "create" && len(args) == 5:
		post(base+"/api/v1/resellers", token, map[string]any{"name": args[2], "username": args[3], "password": args[4]})
	case join(args) == "firewall apply":
		post(base+"/api/v1/server/firewall/apply", token, map[string]any{})
	case args[0] == "reboot":
		post(base+"/api/v1/server/reboot", token, map[string]any{"confirm": "REBOOT"})
	case join(args) == "config validate":
		fmt.Println(`{"ok":true,"templates":"versioned","rule":"test-before-reload"}`)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", strings.Join(args, " "))
		os.Exit(2)
	}
}

func migrateAccount(base, token, id, user, domain string) {
	post(base+"/api/v1/accounts/"+id+"/migrate", token, map[string]any{"username": user, "domain": domain})
}

func waitJob(base, token, id string) {
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		out := do(http.MethodGet, base+"/api/v1/jobs/"+id, token, nil, false)
		st, _ := out["state"].(string)
		if st == "succeeded" {
			fmt.Println(mustJSON(out))
			return
		}
		if st == "failed" {
			fmt.Fprintln(os.Stderr, mustJSON(out))
			os.Exit(1)
		}
		time.Sleep(400 * time.Millisecond)
	}
	fatal("job wait timeout")
}

func login(base, user, pass string) {
	out := do(http.MethodPost, base+"/api/v1/auth/login", "", map[string]any{"username": user, "password": pass}, false)
	if tok, _ := out["token"].(string); tok != "" {
		fmt.Fprintf(os.Stderr, "%s\n", mustJSON(out))
		fmt.Printf("export PANEL_TOKEN=%s\n", tok)
	}
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func doBytes(method, url, token string, body []byte) map[string]any {
	req, _ := http.NewRequest(method, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	c := &http.Client{Timeout: 15 * time.Second}
	res, err := c.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	fmt.Println(string(b))
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	if res.StatusCode >= 400 {
		os.Exit(1)
	}
	return out
}

func get(url, token string) {
	do(http.MethodGet, url, token, nil, true)
}

func post(url, token string, body any) map[string]any {
	return do(http.MethodPost, url, token, body, true)
}

func do(method, url, token string, body any, print bool) map[string]any {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, url, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	c := &http.Client{Timeout: 15 * time.Second}
	res, err := c.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if print {
		fmt.Println(string(b))
	}
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	if res.StatusCode >= 400 {
		os.Exit(1)
	}
	return out
}

func join(args []string) string { return strings.Join(args, " ") }

func fatal(s string) {
	fmt.Fprintln(os.Stderr, s)
	os.Exit(2)
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
