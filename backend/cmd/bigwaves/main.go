// Command bigwaves runs the HTTP API, the embedded frontend and the scheduler in one process.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"bigwaves/internal/app"
	"bigwaves/internal/config"
	"bigwaves/internal/platform/logging"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logging.Setup(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = a.Close() }()

	return a.Serve(ctx)
}
