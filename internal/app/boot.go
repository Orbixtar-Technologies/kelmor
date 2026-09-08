package app

import (
	"context"
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
	Store  *store.Memory
	API    *httpserver.API
	Worker *job.Worker
	Agent  *operations.Host
	Log    *logging.Logger
	Box    *secret.Box
	Root   string
}

func Boot(ctx context.Context, service string) (*Runtime, error) {
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
	st := store.NewMemory()
	if os.Getenv("PANEL_SKIP_SEED") != "1" {
		admin := env("PANEL_ADMIN_USER", "admin")
		pass := env("PANEL_ADMIN_PASSWORD", "ChangeMeOnce!2026")
		email := env("PANEL_ADMIN_EMAIL", "admin@localhost")
		if err := store.SeedDev(st, admin, pass, email); err != nil {
			return nil, err
		}
	}
	agent := &operations.Host{Root: filepath.Join(root, "host")}
	_ = os.MkdirAll(agent.Root, 0o755)
	api := httpserver.New(st, log, agent)
	w := job.New(st, agent, log, hostname())
	rt := &Runtime{Store: st, API: api, Worker: w, Agent: agent, Log: log, Box: box, Root: root}
	log.Info(ctx, "runtime.boot", map[string]any{"service": service, "key_fp": box.Fingerprint(), "state": root})
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
