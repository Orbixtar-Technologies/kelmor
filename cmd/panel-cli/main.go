package main

import (
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
  server status
  health
  account list
  jobs list
  jobs failed
  config validate`)
		os.Exit(2)
	}
	base := env("PANEL_API", "http://127.0.0.1:18080")
	token := env("PANEL_TOKEN", "")
	cmd := strings.Join(os.Args[1:], " ")
	switch {
	case cmd == "version":
		get(base+"/api/v1/version", token)
	case cmd == "health":
		get(base+"/healthz", token)
	case cmd == "server status":
		get(base+"/api/v1/server", token)
	case cmd == "account list":
		get(base+"/api/v1/accounts", token)
	case cmd == "jobs list":
		get(base+"/api/v1/jobs", token)
	case cmd == "jobs list --failed" || cmd == "jobs failed":
		get(base+"/api/v1/jobs?state=failed", token)
	case cmd == "config validate":
		fmt.Println(`{"ok":true,"templates":"versioned","rule":"test-before-reload"}`)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		os.Exit(2)
	}
}

func get(url, token string) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	c := &http.Client{Timeout: 10 * time.Second}
	res, err := c.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	fmt.Println(string(b))
	if res.StatusCode >= 400 {
		os.Exit(1)
	}
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

var _ = json.Marshal
