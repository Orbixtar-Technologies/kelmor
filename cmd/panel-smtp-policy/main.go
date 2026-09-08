package main

import (
	"bufio"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/hosting-panel/panel/internal/mail"
)

func main() {
	addr := env("PANEL_SMTP_POLICY_ADDR", "127.0.0.1:10031")
	limits := env("PANEL_SMTP_LIMITS", "/var/lib/panel/mail/send-limits")
	counts := env("PANEL_SMTP_COUNTS", "/var/lib/panel/mail/send-counts")
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("smtp-policy %s", addr)
	for {
		c, err := ln.Accept()
		if err != nil {
			continue
		}
		go handle(c, limits, counts)
	}
}

func handle(c net.Conn, limits, counts string) {
	defer c.Close()
	attrs := map[string]string{}
	sc := bufio.NewScanner(c)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			break
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		attrs[k] = v
	}
	action, err := mail.Decide(limits, counts, attrs["sender"], attrs["sasl_username"], time.Now().UTC())
	if err != nil {
		action = "DUNNO"
	}
	_, _ = c.Write([]byte(mail.PolicyResponse(action)))
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
