// Package config loads process configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Env      string `env:"APP_ENV" envDefault:"development"`
	HTTPAddr string `env:"HTTP_ADDR" envDefault:":8080"`
	// AppURL is the public origin, used for the OIDC redirect and Origin checks.
	AppURL   string `env:"APP_URL" envDefault:"http://localhost:5173"`
	DBPath   string `env:"DB_PATH" envDefault:"data/bigwaves.db"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	// TradingEnabled gates every write call to an exchange (orders, borrow, repay, transfer).
	TradingEnabled bool `env:"TRADING_ENABLED" envDefault:"false"`
	// SchedulerEnabled turns the in-process cron jobs on or off.
	SchedulerEnabled bool `env:"SCHEDULER_ENABLED" envDefault:"true"`

	Binance   Binance
	Auth0     Auth0
	Discord   Discord
	Session   Session
	SentryDSN string `env:"SENTRY_DSN"`

	// EMA1DAPIToken protects the token endpoints called by external tasks.
	EMA1DAPIToken string `env:"EMA1D_API_TOKEN"`
}

type Binance struct {
	BaseURL   string `env:"BINANCE_BASE_URL" envDefault:"https://api-gcp.binance.com"`
	APIKey    string `env:"BINANCE_API_KEY"`
	SecretKey string `env:"BINANCE_SECRET_KEY"`
}

type Auth0 struct {
	Domain       string `env:"AUTH0_DOMAIN"`
	ClientID     string `env:"AUTH0_CLIENT_ID"`
	ClientSecret string `env:"AUTH0_CLIENT_SECRET"`
}

type Discord struct {
	OrderWebhook  string `env:"DISCORD_ORDER_WEBHOOK"`
	AlertWebhook  string `env:"DISCORD_ALERT_WEBHOOK"`
	ReportWebhook string `env:"DISCORD_REPORT_WEBHOOK"`
}

type Session struct {
	TTL time.Duration `env:"SESSION_TTL" envDefault:"720h"`
	// DevUserEmail signs every request in as this user, without Auth0. Only honored in development.
	DevUserEmail string `env:"DEV_USER_EMAIL"`
}

func (c Config) IsProduction() bool { return c.Env == "production" }

func Load() (Config, error) {
	var c Config
	if err := env.Parse(&c); err != nil {
		return c, fmt.Errorf("parse env: %w", err)
	}
	return c, c.validate()
}

func (c Config) validate() error {
	var errs []error
	if c.DBPath == "" {
		errs = append(errs, errors.New("DB_PATH is required"))
	}
	if c.IsProduction() {
		if c.Auth0.Domain == "" || c.Auth0.ClientID == "" || c.Auth0.ClientSecret == "" {
			errs = append(errs, errors.New("AUTH0_DOMAIN, AUTH0_CLIENT_ID and AUTH0_CLIENT_SECRET are required in production"))
		}
		if c.Session.DevUserEmail != "" {
			errs = append(errs, errors.New("DEV_USER_EMAIL must not be set in production"))
		}
	}
	if c.TradingEnabled && (c.Binance.APIKey == "" || c.Binance.SecretKey == "") {
		errs = append(errs, errors.New("TRADING_ENABLED requires BINANCE_API_KEY and BINANCE_SECRET_KEY"))
	}
	return errors.Join(errs...)
}
