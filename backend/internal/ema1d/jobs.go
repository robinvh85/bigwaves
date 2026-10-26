package ema1d

import (
	"context"
	"fmt"
	"time"

	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/scheduler"
)

// Jobs returns the scheduled jobs of the strategy (times as in the Rails sidekiq_scheduler.yml).
func (s *Service) Jobs() []scheduler.Job {
	return []scheduler.Job{
		{
			Name: "ema1d.process_todos", Description: "EMA1D: thực thi hàng đợi lệnh (CLOSE trước OPEN, retry có backoff)",
			Cron: "20 * * * * *", Timeout: 10 * time.Minute, Run: s.ProcessTodos,
		},
		{
			Name: "ema1d.monitor", Description: "EMA1D: giá, lãi/lỗ chưa chốt, margin level, nhắc khi chưa sync",
			Cron: "CRON_TZ=Asia/Ho_Chi_Minh 40 */15 * * * *", Timeout: 3 * time.Minute, Run: s.Monitor,
		},
		{
			Name: "ema1d.daily_check", Description: "EMA1D: nến → tín hiệu → sync → báo cáo Discord (07:03 VN)",
			Cron: "CRON_TZ=Asia/Ho_Chi_Minh 0 3 7 * * *", Timeout: 15 * time.Minute, Run: s.dailyCheckJob,
		},
		{
			Name: "ema1d.daily_snapshot", Description: "EMA1D: ảnh chụp equity hằng ngày + kiểm tra pair trên Binance (07:05 VN)",
			Cron: "CRON_TZ=Asia/Ho_Chi_Minh 0 5 7 * * *", Timeout: 10 * time.Minute, Run: s.Snapshot,
		},
		{
			Name: "ema1d.cleanup", Description: "EMA1D: xoá các sync dry-run cũ hơn 30 ngày",
			Cron: "CRON_TZ=Asia/Ho_Chi_Minh 0 15 3 * * *", Timeout: time.Minute,
			Run: func(ctx context.Context) (string, error) {
				n, err := dbq.New(s.DB.Write).DeleteDryRunSyncsBefore(ctx, dbtype.NewTime(s.now().AddDate(0, 0, -30)))
				return fmt.Sprintf("%d dry-run sync(s) deleted", n), err
			},
		},
	}
}
