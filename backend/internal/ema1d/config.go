package ema1d

import (
	"errors"
	"fmt"

	"bigwaves/internal/platform/money"
)

// StrategyKey is the config store key of this strategy.
const StrategyKey = "ema1d"

// Sizes are % of the strategy equity per pair group and side.
type Sizes struct {
	CoreLong       money.Dec `json:"coreLong"`
	CoreShort      money.Dec `json:"coreShort"`
	SensitiveLong  money.Dec `json:"sensitiveLong"`
	SensitiveShort money.Dec `json:"sensitiveShort"`
}

// Config replaces the EMA1D_* AppSettings of the Rails app.
type Config struct {
	// Enabled is the main switch for executing todos (EMA1D_ENABLED).
	Enabled bool `json:"enabled"`
	// PauseOpen blocks new entries from the sync, manual todos and the executor; closes still run.
	PauseOpen bool `json:"pauseOpen"`
	// DailyCheckEnabled runs the 07:03 VN daily check (candles → signals → sync → report).
	DailyCheckEnabled bool `json:"dailyCheckEnabled"`
	// BudgetUSDT is the capital of the strategy; sizes use it plus the realized PnL of EMA1D
	// positions, never the account balance (other strategies share the cross account).
	BudgetUSDT money.Dec `json:"budgetUsdt"`
	// MaxEntryAgeDays: entries older than this many days are not opened.
	MaxEntryAgeDays int       `json:"maxEntryAgeDays"`
	Sizes           Sizes     `json:"sizes"`
	MarginLevelWarn money.Dec `json:"marginLevelWarn"`
	// MarginLevelAlert is the stronger alert threshold.
	MarginLevelAlert money.Dec `json:"marginLevelAlert"`
}

// SuggestedSizes are the sizes the backtest was run with (configuration "4d"). A size set below
// them is treated as a test run: an order under Binance's minimum is raised to the minimum.
var SuggestedSizes = Sizes{
	CoreLong:       money.MustParse("7"),
	CoreShort:      money.MustParse("3.5"),
	SensitiveLong:  money.MustParse("2.5"),
	SensitiveShort: money.MustParse("1.25"),
}

func DefaultConfig() Config {
	return Config{
		BudgetUSDT:       money.FromInt(6000),
		MaxEntryAgeDays:  1,
		Sizes:            SuggestedSizes,
		MarginLevelWarn:  money.MustParse("1.5"),
		MarginLevelAlert: money.MustParse("1.3"),
	}
}

func (c Config) Validate() error {
	var errs []error
	if c.BudgetUSDT.IsNegative() {
		errs = append(errs, errors.New("budgetUsdt must not be negative"))
	}
	if c.MaxEntryAgeDays < 0 || c.MaxEntryAgeDays > 30 {
		errs = append(errs, errors.New("maxEntryAgeDays must be between 0 and 30"))
	}
	for name, v := range map[string]money.Dec{
		"coreLong": c.Sizes.CoreLong, "coreShort": c.Sizes.CoreShort,
		"sensitiveLong": c.Sizes.SensitiveLong, "sensitiveShort": c.Sizes.SensitiveShort,
	} {
		if v.IsNegative() || v.GreaterThan(money.FromInt(100)) {
			errs = append(errs, fmt.Errorf("sizes.%s must be between 0 and 100", name))
		}
	}
	if c.MarginLevelAlert.GreaterThan(c.MarginLevelWarn) {
		errs = append(errs, errors.New("marginLevelAlert must not be above marginLevelWarn"))
	}
	return errors.Join(errs...)
}

// SizeFor returns the configured and the suggested % for a group and side.
func (c Config) SizeFor(group, side string) (configured, suggested money.Dec) {
	switch {
	case group == GroupSensitive && side == SideLong:
		return c.Sizes.SensitiveLong, SuggestedSizes.SensitiveLong
	case group == GroupSensitive:
		return c.Sizes.SensitiveShort, SuggestedSizes.SensitiveShort
	case side == SideLong:
		return c.Sizes.CoreLong, SuggestedSizes.CoreLong
	default:
		return c.Sizes.CoreShort, SuggestedSizes.CoreShort
	}
}
