package emasettings

import (
	"time"

	"bigwaves/internal/platform/scheduler"
)

// Jobs returns the 5-minute scan (Rails: EmaSettings::CheckTradesService in Each5MinutesSchedule).
// Times are UTC: the D1 window opens at 00:00 UTC.
func (s *Service) Jobs() []scheduler.Job {
	return []scheduler.Job{{
		Name:        "emasettings.check",
		Description: "EMA Settings: stop loss / target / EMA cross back, new trade on the cross, open waiting trades, PnL, setting TP/SL",
		Cron:        "50 */5 * * * *", Timeout: 4 * time.Minute, Run: s.Check,
	}}
}
