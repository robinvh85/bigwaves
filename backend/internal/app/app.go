package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"bigwaves/internal/config"
	"bigwaves/internal/platform/db"
)

type App struct {
	Cfg config.Config
	DB  *db.DB
}

// New opens the database and applies migrations.
func New(ctx context.Context, cfg config.Config) (*App, error) {
	d, err := db.Open(ctx, cfg.DBPath)
	if err != nil {
		return nil, err
	}
	if err := d.Migrate(ctx); err != nil {
		_ = d.Close()
		return nil, err
	}
	return &App{Cfg: cfg, DB: d}, nil
}

func (a *App) Close() error { return a.DB.Close() }

// Serve runs the HTTP server until ctx is cancelled, then shuts down gracefully.
func (a *App) Serve(ctx context.Context) error {
	srv := &http.Server{
		Addr:              a.Cfg.HTTPAddr,
		Handler:           a.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		slog.Info("http listening", "addr", a.Cfg.HTTPAddr, "trading_enabled", a.Cfg.TradingEnabled)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
