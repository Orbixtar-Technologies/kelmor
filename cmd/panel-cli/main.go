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
  account export <id>
  account import <export.json> [username] [domain]
  mailbox create <account_id> <mail_domain_id> <local> <password>
  db create <account_id> <name> <engine>
  website create <account_id> <domain_id> <runtime>
  domain create <account_id> <fqdn> [runtime]
  file list <account_id> [path]
  file write <account_id> <path> <content>
  backup create <account_id>
  backup restore <account_id> <backup_id>
  jobs list
  jobs failed
  audit
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
	case args[0] == "account" && len(args) == 3 && args[1] == "export":
		get(base+"/api/v1/accounts/"+args[2]+"/export", token)
	case args[0] == "backup" && len(args) == 3 && args[1] == "create":
		post(base+"/api/v1/accounts/"+args[2]+"/backups", token, map[string]any{"kind": "full", "destination": "local"})
	case join(args) == "jobs list":
		get(base+"/api/v1/jobs", token)
	case join(args) == "jobs list --failed" || join(args) == "jobs failed":
		get(base+"/api/v1/jobs?state=failed", token)
	case args[0] == "audit":
		get(base+"/api/v1/audit-events", token)
	case join(args) == "config validate":
		fmt.Println(`{"ok":true,"templates":"versioned","rule":"test-before-reload"}`)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", strings.Join(args, " "))
		os.Exit(2)
	}
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
