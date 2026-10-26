package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"

	"bigwaves/internal/account"
	"bigwaves/internal/auth"
	"bigwaves/internal/config"
	"bigwaves/internal/ema1d"
	"bigwaves/internal/exchange/binance"
	"bigwaves/internal/marketdata"
	"bigwaves/internal/platform/configstore"
	"bigwaves/internal/platform/db"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/events"
	"bigwaves/internal/platform/notify"
	"bigwaves/internal/platform/scheduler"
)

type App struct {
	Cfg       config.Config
	DB        *db.DB
	Binance   *binance.Client
	Notifier  notify.Notifier
	Alerter   *notify.Alerter
	Events    *events.Broker
	Configs   *configstore.Store
	Scheduler *scheduler.Scheduler
	Auth      *auth.Service
	Pairs     *marketdata.Service
	Account   *account.Service
	EMA1D     *ema1d.Service
}

// New opens the database, applies migrations and builds every service.
func New(ctx context.Context, cfg config.Config) (*App, error) {
	if cfg.SentryDSN != "" {
		if err := sentry.Init(sentry.ClientOptions{Dsn: cfg.SentryDSN, Environment: cfg.Env}); err != nil {
			return nil, fmt.Errorf("sentry: %w", err)
		}
	}
	d, err := db.Open(ctx, cfg.DBPath)
	if err != nil {
		return nil, err
	}
	if err := d.Migrate(ctx); err != nil {
		_ = d.Close()
		return nil, err
	}

	a := &App{Cfg: cfg, DB: d}
	a.Binance = binance.New(binance.Config{
		BaseURL: cfg.Binance.BaseURL, APIKey: cfg.Binance.APIKey, SecretKey: cfg.Binance.SecretKey,
		TradingEnabled: cfg.TradingEnabled,
	})
	a.Notifier = notify.NewDiscord(cfg.Discord.OrderWebhook, cfg.Discord.AlertWebhook, cfg.Discord.ReportWebhook)
	a.Alerter = notify.NewAlerter(d, a.Notifier)
	a.Events = events.NewBroker()
	a.Configs = configstore.New(d)
	a.Scheduler = scheduler.New(d, a.Alerter, a.Events)
	a.Auth = auth.New(cfg, d)
	a.Pairs = &marketdata.Service{DB: d, Binance: a.Binance}
	a.Account = &account.Service{DB: d, Binance: a.Binance}

	a.EMA1D = &ema1d.Service{
		DB: d, Binance: a.Binance, Configs: a.Configs, Notifier: a.Notifier, Events: a.Events, Pairs: a.Pairs,
		ReportConfigured: cfg.Discord.ReportWebhook != "",
	}

	a.Scheduler.Register(a.Pairs.Jobs(a.Events)...)
	a.Scheduler.Register(a.EMA1D.Jobs()...)
	a.Scheduler.Register(a.maintenanceJob())
	return a, nil
}

func (a *App) maintenanceJob() scheduler.Job {
	return scheduler.Job{
		Name: "maintenance.retention", Description: "Xoá session hết hạn và lịch sử job cũ hơn 30 ngày",
		Cron: "CRON_TZ=Asia/Ho_Chi_Minh 0 0 3 * * *", Timeout: 5 * time.Minute,
		Run: func(ctx context.Context) (string, error) {
			sessions, err := a.Auth.CleanupExpired(ctx)
			if err != nil {
				return "", err
			}
			runs, err := dbq.New(a.DB.Write).DeleteJobRunsBefore(ctx, dbtype.NewTime(time.Now().AddDate(0, 0, -30)))
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%d sessions, %d job runs deleted", sessions, runs), nil
		},
	}
}

func (a *App) Close() error {
	sentry.Flush(2 * time.Second)
	return a.DB.Close()
}

// Serve runs the HTTP server and the scheduler until ctx is cancelled, then shuts down: stop
// accepting requests, wait for running jobs, close the database.
func (a *App) Serve(ctx context.Context) error {
	if a.Cfg.SchedulerEnabled {
		if err := a.Scheduler.Start(ctx); err != nil {
			return err
		}
	} else {
		slog.Warn("scheduler disabled (SCHEDULER_ENABLED=false)")
	}

	srv := &http.Server{
		Addr:              a.Cfg.HTTPAddr,
		Handler:           a.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		slog.Info("http listening", "addr", a.Cfg.HTTPAddr, "trading_enabled", a.Cfg.TradingEnabled, "env", a.Cfg.Env)
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
	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := srv.Shutdown(shutdownCtx)
	return errors.Join(err, a.Scheduler.Stop())
}
