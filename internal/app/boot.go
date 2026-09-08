package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/httpserver"
	"github.com/hosting-panel/panel/internal/job"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/pkg/secret"
	"github.com/hosting-panel/panel/internal/store"
)

type Runtime struct {
	Store  store.Store
	API    *httpserver.API
	Worker *job.Worker
	Agent  *operations.Host
	Log    *logging.Logger
	Box    *secret.Box
	Root   string
}

func Boot(ctx context.Context, service string) (*Runtime, error) {
	if (service == "panel-api" || service == "panel-worker") && os.Geteuid() == 0 && os.Getenv("PANEL_ALLOW_ROOT") != "1" {
		return nil, fmt.Errorf("%s must run as the unprivileged panel user, not root", service)
	}
	root := os.Getenv("PANEL_STATE_DIR")
	if root == "" {
		root = filepath.Join("var", "panel")
	}
	if err := os.MkdirAll(filepath.Join(root, "secrets"), 0o750); err != nil {
		return nil, err
	}
	box, err := secret.LoadOrCreate(filepath.Join(root, "secrets", "master.key"))
	if err != nil {
		return nil, err
	}
	log := logging.New(service)
	dsn := os.Getenv("PANEL_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres:///panel_control?host=/var/run/postgresql"
	}
	pg, err := store.OpenPostgres(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("control database: %w", err)
	}
	pg.SeedDatabaseServers()
	if os.Getenv("PANEL_SKIP_SEED") != "1" {
		admin := env("PANEL_ADMIN_USER", "admin")
		pass := env("PANEL_ADMIN_PASSWORD", "ChangeMeOnce!2026")
		email := env("PANEL_ADMIN_EMAIL", "admin@localhost")
		if err := store.SeedDev(pg, admin, pass, email); err != nil {
			return nil, err
		}
	}
	agent := &operations.Host{Sock: os.Getenv("PANEL_AGENT_SOCK")}
	if agent.Sock == "" {
		agent.Root = filepath.Join(root, "host")
		_ = os.MkdirAll(agent.Root, 0o755)
	}
	api := httpserver.New(pg, log, agent)
	w := job.New(pg, agent, log, box, hostname())
	rt := &Runtime{Store: pg, API: api, Worker: w, Agent: agent, Log: log, Box: box, Root: root}
	log.Info(ctx, "runtime.boot", map[string]any{"service": service, "key_fp": box.Fingerprint(), "state": root, "database": "postgresql"})
	return rt, nil
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func hostname() string {
	h, _ := os.Hostname()
	if h == "" {
		return "panel-worker"
	}
	return h
}
