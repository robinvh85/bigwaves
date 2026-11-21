package emacross

import (
	"context"
	"fmt"
	"time"

	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/scheduler"
)

// Jobs returns the D1 check (minute 03 of every hour, as in the Rails schedule) and the alert
// cleanup.
func (s *Service) Jobs() []scheduler.Job {
	return []scheduler.Job{
		{
			Name: "emacross.check_d1", Description: "EMA cross: alert when EMA14 crosses Smooth10 on D1",
			Cron: "30 3 * * * *", Timeout: 5 * time.Minute, Run: s.Check,
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
