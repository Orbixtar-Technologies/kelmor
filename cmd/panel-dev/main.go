package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hosting-panel/panel/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	rt, err := app.Boot(ctx, "panel-dev")
	if err != nil {
		log.Fatal(err)
	}
	go rt.Worker.Run(ctx)
	addr := os.Getenv("PANEL_API_ADDR")
	if addr == "" {
		addr = "127.0.0.1:18080"
	}
	srv := &http.Server{Addr: addr, Handler: rt.API.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		rt.Log.Info(ctx, "dev.listen", map[string]any{"addr": addr})
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(sctx)
}
