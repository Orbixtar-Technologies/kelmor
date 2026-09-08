package main

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"net"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"syscall"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/pkg/logging"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	sock := os.Getenv("PANEL_AGENT_SOCK")
	if sock == "" {
		sock = "/run/panel/agent.sock"
		if os.Getenv("PANEL_DEV") == "1" {
			sock = "var/panel/run/agent.sock"
		}
	}
	_ = os.MkdirAll(dir(sock), 0o751)
	// nginx (www-data) must traverse to /run/panel/apps/*.sock;
	// the agent socket itself stays 0660 root:panel.
	_ = os.Chmod(dir(sock), 0o751)
	_ = os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		log.Fatal(err)
	}
	_ = os.Chmod(sock, 0o660)
	for _, name := range []string{"panel", "ubuntu"} {
		if g, err := user.LookupGroup(name); err == nil {
			if gid, err := strconv.Atoi(g.Gid); err == nil {
				_ = os.Chown(sock, 0, gid)
				break
			}
		}
	}
	logg := logging.New("panel-agent")
	host := &operations.Host{Root: os.Getenv("PANEL_HOST_ROOT")}
	logg.Info(ctx, "agent.listen", map[string]any{"socket": sock})
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go serve(c, host, logg)
	}
}

func serve(c net.Conn, host *operations.Host, logg *logging.Logger) {
	defer c.Close()
	sc := bufio.NewScanner(c)
	sc.Buffer(make([]byte, 0, 64*1024), operations.MaxRPCLine)
	for sc.Scan() {
		var req operations.Request
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			enc(c, map[string]any{"ok": false, "error": err.Error()})
			continue
		}
		res, err := host.Dispatch(context.Background(), req)
		if err != nil {
			enc(c, map[string]any{"ok": false, "error": err.Error()})
			continue
		}
		enc(c, map[string]any{"ok": true, "result": res})
	}
}

func enc(c net.Conn, v any) {
	b, _ := json.Marshal(v)
	_, _ = c.Write(append(b, '\n'))
}

func dir(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return "."
}
