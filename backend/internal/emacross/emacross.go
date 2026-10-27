// Package emacross sends a Discord alert when EMA14 of the close crosses Smooth10 (EMA10 of the
// EMA) on H1, H4 or D1 for the watched pairs. It never trades. Ported from the Rails
// CheckCross{H1,H4,D1}EmasForPairsService with three changes: the averages come from Binance
// klines (no ema_trackings table to seed by hand), a cross is sent once per candle instead of on
// every run while it holds, and each timeframe runs from one job only (D1 was sent twice).
package emacross

import (
	"context"
	"errors"
	"fmt"
	"time"

	"bigwaves/internal/exchange/binance"
	"bigwaves/internal/platform/configstore"
	"bigwaves/internal/platform/db"
	"bigwaves/internal/platform/events"
	"bigwaves/internal/platform/httpx"
	"bigwaves/internal/platform/money"
	"bigwaves/internal/platform/notify"
)

// StrategyKey is the config store key of the alerts.
const StrategyKey = "emacross"

// Timeframe is one candle size the alert runs on.
type Timeframe struct {
	Name string // H1, H4, D1
	API  string // Binance kline interval
	Step time.Duration
}

var (
	H1 = Timeframe{"H1", "1h", time.Hour}
	H4 = Timeframe{"H4", "4h", 4 * time.Hour}
	D1 = Timeframe{"D1", "1d", 24 * time.Hour}

	Timeframes = []Timeframe{H1, H4, D1}
)

// Thresholds are, per timeframe, how far in % EMA14 must pass Smooth10 to count as a cross
// (OVER_CROSS_EMA_PERCENT_FOR_OPEN in Rails).
type Thresholds struct {
	H1 money.Dec `json:"h1"`
	H4 money.Dec `json:"h4"`
	D1 money.Dec `json:"d1"`
}

// Config of the alerts.
type Config struct {
	// Enabled is the main switch: off, the jobs skip.
	Enabled          bool       `json:"enabled"`
	ThresholdPercent Thresholds `json:"thresholdPercent"`
}

func DefaultConfig() Config {
	p := money.MustParse("0.01")
	return Config{Enabled: true, ThresholdPercent: Thresholds{H1: p, H4: p, D1: p}}
}

func (c Config) Validate() error {
	var errs []error
	for _, t := range []struct {
		name string
		v    money.Dec
	}{{"h1", c.ThresholdPercent.H1}, {"h4", c.ThresholdPercent.H4}, {"d1", c.ThresholdPercent.D1}} {
		if t.v.IsNegative() || t.v.GreaterThan(money.FromInt(100)) {
			errs = append(errs, fmt.Errorf("thresholdPercent.%s must be between 0 and 100", t.name))
		}
	}
	return errors.Join(errs...)
}

// Threshold returns the % for a timeframe.
func (c Config) Threshold(tf Timeframe) money.Dec {
	switch tf {
	case H1:
		return c.ThresholdPercent.H1
	case H4:
		return c.ThresholdPercent.H4
	default:
		return c.ThresholdPercent.D1
	}
}

type Service struct {
	DB       *db.DB
	Binance  *binance.Client
	Configs  *configstore.Store
	Notifier notify.Notifier
	Events   *events.Broker
	Now      func() time.Time
	// Sleep paces the kline calls; tests replace it.
	Sleep func(time.Duration)
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) sleep(d time.Duration) {
	if s.Sleep != nil {
		s.Sleep(d)
		return
	}
	time.Sleep(d)
}

func (s *Service) Config(ctx context.Context) (Config, int64, error) {
	c := DefaultConfig()
	v, err := s.Configs.Load(ctx, StrategyKey, &c)
	return c, v, err
}

func (s *Service) SaveConfig(ctx context.Context, version int64, c Config, user string) (int64, error) {
	if err := c.Validate(); err != nil {
		return 0, httpx.Validation(err.Error(), nil)
	}
	v, err := s.Configs.Save(ctx, StrategyKey, version, c, user)
	if err == nil {
		s.changed()
	}
	return v, err
}

func (s *Service) changed() { s.Events.Publish("emacross.updated", nil) }
