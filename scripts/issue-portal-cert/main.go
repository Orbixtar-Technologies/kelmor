package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/acme"
)

func main() {
	host := strings.TrimSpace(os.Getenv("PANEL_HOSTNAME"))
	if host == "" {
		host = "lab.kelmor.host"
	}
	directory := acme.Directory()
	if directory == "" {
		fmt.Fprintln(os.Stderr, "ACME directory not configured")
		os.Exit(1)
	}
	contact := strings.TrimSpace(os.Getenv("PANEL_ADMIN_EMAIL"))
	if contact == "" {
		contact = "admin@" + host
	}
	names := acme.HostnamesForPortal(host)
	fmt.Fprintf(os.Stderr, "issuing %v via %s\n", names, directory)
	agent := &operations.Host{Sock: strings.TrimSpace(os.Getenv("PANEL_AGENT_SOCK"))}
	exp, err := acme.IssueNames(context.Background(), agent, names, contact, directory)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("issued until %s\n", exp.UTC().Format("2006-01-02"))
}
