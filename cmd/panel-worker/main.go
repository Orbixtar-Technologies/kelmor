package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/hosting-panel/panel/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	rt, err := app.Boot(ctx, "panel-worker")
	if err != nil {
		log.Fatal(err)
	}
	rt.Log.Info(ctx, "worker.start", nil)
	rt.Worker.Run(ctx)
}
