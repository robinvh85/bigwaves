// Package emasettings runs EMA Settings: budgeted campaigns that trade the EMA14 / Smooth10 cross
// of one pair on D1 or H4, with a stop loss and target per trade and a take profit / stop loss on
// the campaign's total PnL. Ported from the Rails EmaSettings::CheckTradesService (see
// docs/ema-settings.md) with these changes: the averages come from Binance klines (no
// ema_trackings), closing a campaign never wraps exchange orders in a DB transaction, one failing
// setting does not stop the others, and PnL is net of fees and interest.
package emasettings

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"bigwaves/internal/exchange/binance"
	"bigwaves/internal/platform/configstore"
	"bigwaves/internal/platform/db"
	"bigwaves/internal/platform/events"
	"bigwaves/internal/platform/httpx"
	"bigwaves/internal/platform/money"
	"bigwaves/internal/platform/notify"
)

// StrategyKey is the config store key and the exchange_orders strategy.
const StrategyKey = "emasettings"

const (
	StatusActive   = "ACTIVE"
	StatusInactive = "INACTIVE"
	StatusDone     = "DONE"

	OnlyLong  = "ONLY_LONG"
	OnlyShort = "ONLY_SHORT"
	Both      = "BOTH"

	PeriodD1 = "D1"
	PeriodH4 = "H4"

	CheckAtOpen    = "CHECK_AT_OPEN"
	CheckAtClose   = "CHECK_AT_CLOSE"
	CheckAtCurrent = "CHECK_AT_CURRENT"

	NewTrade    = "NEW_TRADE"
	NotNewTrade = "NOT_NEW_TRADE"

	SideLong  = "LONG"
	SideShort = "SHORT"

	TradeOpening = "OPENING" // waiting to enter, nothing on the exchange yet
	TradeOpen    = "OPEN"    // held (Rails: CLOSING)
	TradeClosed  = "CLOSED"  // Rails: DONE

	ReasonSL         = "SL"
	ReasonTarget     = "TARGET"
	ReasonEMAReverse = "EMA_REVERSE"
	ReasonSettingTP  = "SETTING_TP"
	ReasonSettingSL  = "SETTING_SL"
	ReasonManual     = "MANUAL"
	ReasonCancelled  = "CANCELLED"
)

// PeriodParams are the thresholds of one timeframe, in percent (the Rails EMA_{D1,H4}_* settings).
type PeriodParams struct {
	// OpenPercent: EMA14 must be this far past Smooth10 to enter (OVER_CROSS_EMA_PERCENT_FOR_OPEN).
	OpenPercent money.Dec `json:"openPercent"`
	// ClosePercent: a cross the other way this far closes the trade (OVER_CROSS_EMA_PERCENT_FOR_CLOSE).
	ClosePercent money.Dec `json:"closePercent"`
	// IgnoreOverEmaPercent: no entry when the price is this far from EMA14 (no chasing).
	IgnoreOverEmaPercent money.Dec `json:"ignoreOverEmaPercent"`
	// StopLossPercent from the entry price, set on each trade when it opens (LOSS_LIMIT_PERCENT).
	StopLossPercent money.Dec `json:"stopLossPercent"`
}

// Config of the strategy.
type Config struct {
	// Enabled is the main switch: off, the scan skips (manual actions still work).
	Enabled bool         `json:"enabled"`
	D1      PeriodParams `json:"d1"`
	H4      PeriodParams `json:"h4"`
}

// DefaultConfig holds the values Rails used in production (2026-10). The switch starts off.
func DefaultConfig() Config {
	return Config{
		D1: PeriodParams{OpenPercent: money.MustParse("0.03"), ClosePercent: money.MustParse("0.03"),
			IgnoreOverEmaPercent: money.FromInt(4), StopLossPercent: money.FromInt(8)},
		H4: PeriodParams{OpenPercent: money.MustParse("0.03"), ClosePercent: money.MustParse("0.03"),
			IgnoreOverEmaPercent: money.FromInt(3), StopLossPercent: money.FromInt(5)},
	}
}

func (c Config) Validate() error {
	var errs []error
	for _, p := range []struct {
		name string
		v    PeriodParams
	}{{"d1", c.D1}, {"h4", c.H4}} {
		for _, f := range []struct {
			name string
			v    money.Dec
		}{
			{"openPercent", p.v.OpenPercent}, {"closePercent", p.v.ClosePercent},
			{"ignoreOverEmaPercent", p.v.IgnoreOverEmaPercent}, {"stopLossPercent", p.v.StopLossPercent},
		} {
			if f.v.IsNegative() || f.v.GreaterThan(money.FromInt(100)) {
				errs = append(errs, fmt.Errorf("%s.%s must be between 0 and 100", p.name, f.name))
			}
		}
	}
	return errors.Join(errs...)
}

// Params returns the thresholds of a period.
func (c Config) Params(period string) PeriodParams {
	if period == PeriodH4 {
		return c.H4
	}
	return c.D1
}

type Service struct {
	DB       *db.DB
	Binance  *binance.Client
	Configs  *configstore.Store
	Notifier notify.Notifier
	Alerter  *notify.Alerter
	Events   *events.Broker
	Now      func() time.Time
	// Sleep waits between account reads after an order; tests replace it.
	Sleep func(time.Duration)

	// mu serializes the scan and the manual actions, so a button and the 5-minute job never work
	// the same trade at once.
	mu sync.Mutex
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

func (s *Service) changed() { s.Events.Publish("emasettings.updated", nil) }
