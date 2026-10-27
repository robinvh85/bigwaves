package emacross

import (
	"context"
	"fmt"
	"time"

	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/scheduler"
)

// Jobs returns one check per timeframe (times as in the Rails schedules: H1 at minutes 01 and 31,
// H4 and D1 at minute 03 of every hour) and the alert cleanup.
func (s *Service) Jobs() []scheduler.Job {
	check := func(tf Timeframe) func(context.Context) (string, error) {
		return func(ctx context.Context) (string, error) { return s.Check(ctx, tf) }
	}
	return []scheduler.Job{
		{
			Name: "emacross.check_h1", Description: "EMA cross: alert when EMA14 crosses Smooth10 on H1",
			Cron: "0 1,31 * * * *", Timeout: 5 * time.Minute, Run: check(H1),
		},
		{
			Name: "emacross.check_h4", Description: "EMA cross: alert when EMA14 crosses Smooth10 on H4",
			Cron: "0 3 * * * *", Timeout: 5 * time.Minute, Run: check(H4),
		},
		{
			Name: "emacross.check_d1", Description: "EMA cross: alert when EMA14 crosses Smooth10 on D1",
			Cron: "30 3 * * * *", Timeout: 5 * time.Minute, Run: check(D1),
		},
		{
			Name: "emacross.cleanup", Description: "EMA cross: delete alerts older than 90 days",
			Cron: "CRON_TZ=Asia/Ho_Chi_Minh 0 20 3 * * *", Timeout: time.Minute,
			Run: func(ctx context.Context) (string, error) {
				n, err := dbq.New(s.DB.Write).DeleteEmacrossAlertsBefore(ctx, dbtype.NewTime(s.now().AddDate(0, 0, -90)))
				return fmt.Sprintf("%d alert(s) deleted", n), err
			},
		},
	}
}
