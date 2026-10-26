package ema1d

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/notify"
	"bigwaves/internal/platform/scheduler"
)

// Daily check statuses.
const (
	CheckPending = "PENDING"
	CheckSynced  = "SYNCED" // signals synced, report not sent yet
	CheckSent    = "SENT"
	CheckFailed  = "FAILED"
)

type DailyCheckOptions struct {
	// DryRun syncs as a dry run and stores no daily check row; it sends only with Deliver.
	DryRun  bool
	Deliver bool
	// Force runs although the daily check is switched off.
	Force bool
}

type DailyCheckResult struct {
	Status   string `json:"status"`
	Date     string `json:"date"`
	RunID    string `json:"runId,omitempty"`
	Attempts int64  `json:"attempts,omitempty"`
	Report   string `json:"report,omitempty"`
	Error    string `json:"error,omitempty"`
}

// DailyCheck does what Claude's scheduled task used to do, inside the bot:
//  1. update the 1D/4H candles of the tracked pairs
//  2. compute where the strategy stands (port of status.py)
//  3. sync that state (run_id "auto-YYYYMMDD": a retry replays, never syncs twice)
//  4. send the report to Discord
//
// Once per candle date (UTC). No automatic retry: a failure is alerted and the day waits for a
// manual run, which redoes everything before the sync, or only resends the stored report after it.
func (s *Service) DailyCheck(ctx context.Context, opt DailyCheckOptions) (DailyCheckResult, error) {
	cfg, _, err := s.Config(ctx)
	if err != nil {
		return DailyCheckResult{}, err
	}
	now := s.now()
	date := dateOnly(now)
	if !opt.DryRun && !opt.Force && !cfg.DailyCheckEnabled {
		return DailyCheckResult{Status: "disabled", Date: date}, nil
	}

	q := dbq.New(s.DB.Write)
	check := dbq.Ema1dDailyCheck{SignalDate: date, Status: CheckPending, Rows: dbtype.JSON("[]"), SyncParams: dbtype.JSON("{}"), SyncResult: dbtype.JSON("{}")}
	if !opt.DryRun {
		if err := q.EnsureDailyCheck(ctx, date); err != nil {
			return DailyCheckResult{}, err
		}
		if check, err = q.GetDailyCheck(ctx, date); err != nil {
			return DailyCheckResult{}, err
		}
		if check.Status == CheckSent {
			return s.checkResult(check, "already_sent", ""), nil
		}
	}
	check.Attempts++
	if err := s.saveCheck(ctx, check, opt.DryRun); err != nil {
		return DailyCheckResult{}, err
	}

	runErr := func() error {
		if check.SyncedAt == nil {
			if err := s.prepareCheck(ctx, &check, cfg, opt.DryRun); err != nil {
				return err
			}
		}
		if opt.Deliver || !opt.DryRun {
			return s.sendReport(ctx, &check, opt.DryRun)
		}
		return nil
	}()
	if runErr != nil {
		s.failCheck(ctx, &check, runErr, opt.DryRun)
		return s.checkResult(check, "failed", runErr.Error()), nil
	}
	s.changed()
	return s.checkResult(check, strings.ToLower(check.Status), ""), nil
}

type checkFailure struct{ msg string }

func (e *checkFailure) Error() string { return e.msg }

func (s *Service) prepareCheck(ctx context.Context, check *dbq.Ema1dDailyCheck, cfg Config, dryRun bool) error {
	candles, err := s.UpdateCandles(ctx)
	if err != nil {
		return err
	}
	if len(candles.Errors) > 0 {
		return &checkFailure{"loading Binance candles: " + strings.Join(candles.Errors[:min(3, len(candles.Errors))], "; ")}
	}
	sig, err := s.Signals(ctx)
	if err != nil {
		return err
	}
	if len(sig.Warnings) > 0 {
		return &checkFailure{"data not ready, not synced: " + strings.Join(sig.Warnings, "; ")}
	}
	date := check.SignalDate
	runID := "auto-" + strings.ReplaceAll(date, "-", "")
	dry := "0"
	if dryRun {
		dry = "1"
	}
	res, err := s.Sync(ctx, SyncRequest{Date: sig.Date, RunID: runID, S: sig.S, DryRun: dry}, "source=daily_check&run_id="+runID)
	if err != nil {
		return err
	}
	st, err := s.Status(ctx)
	if err != nil {
		return err
	}
	bot := botLines(res, st)
	if dryRun {
		bot[0] += " (DRY RUN)"
	}
	notes := make([]string, len(candles.Gaps))
	for i, g := range candles.Gaps {
		notes[i] = "missing candles: " + g
	}
	rows, _ := json.Marshal(sig.Rows)
	params, _ := json.Marshal(map[string]any{"date": sig.Date, "s": sig.S, "warnings": sig.Warnings})
	result, _ := json.Marshal(res)
	check.RunID, check.Rows, check.SyncParams, check.SyncResult = runID, dbtype.JSON(rows), dbtype.JSON(params), dbtype.JSON(result)
	check.Report = reportText(date, sig.Rows, bot, notes)
	check.Status, check.SyncedAt, check.LastError = CheckSynced, tptr(s.now()), ""
	return s.saveCheck(ctx, *check, dryRun)
}

func (s *Service) sendReport(ctx context.Context, check *dbq.Ema1dDailyCheck, dryRun bool) error {
	if !s.ReportConfigured {
		return &checkFailure{"DISCORD_REPORT_WEBHOOK is not set"}
	}
	parts := reportParts(check.Report, 1900)
	for i, part := range parts {
		if i > 0 {
			s.sleep(time.Second)
		}
		if err := s.Notifier.Send(ctx, notify.Reports, part); err != nil {
			return &checkFailure{fmt.Sprintf("sending to Discord failed (part %d): %v", i+1, err)}
		}
	}
	check.Status, check.SentAt, check.LastError = CheckSent, tptr(s.now()), ""
	return s.saveCheck(ctx, *check, dryRun)
}

func (s *Service) failCheck(ctx context.Context, check *dbq.Ema1dDailyCheck, err error, dryRun bool) {
	msg := err.Error()
	var cf *checkFailure
	if !errors.As(err, &cf) {
		msg = fmt.Sprintf("%T: %s", err, err.Error())
	}
	slog.ErrorContext(ctx, "ema1d daily check failed", "date", check.SignalDate, "err", msg)
	check.Status, check.LastError = CheckFailed, truncate(msg, 2000)
	if saveErr := s.saveCheck(ctx, *check, dryRun); saveErr != nil {
		slog.ErrorContext(ctx, "record daily check failure", "err", saveErr)
	}
	if dryRun {
		return
	}
	step := "chưa sync"
	if check.SyncedAt != nil {
		step = "đã sync, chưa gửi báo cáo"
	}
	s.notify(ctx, fmt.Sprintf("EMA1D check %s: LỖI (%s) — %s. Chạy lại: màn EMA1D → Daily check, hoặc `bwctl job run ema1d.daily_check`",
		check.SignalDate, step, truncate(msg, 500)))
}

func (s *Service) saveCheck(ctx context.Context, c dbq.Ema1dDailyCheck, dryRun bool) error {
	if dryRun {
		return nil
	}
	return dbq.New(s.DB.Write).SaveDailyCheck(ctx, dbq.SaveDailyCheckParams{
		Status: c.Status, RunID: c.RunID, Attempts: c.Attempts, Rows: c.Rows, SyncParams: c.SyncParams,
		SyncResult: c.SyncResult, Report: c.Report, LastError: c.LastError, SyncedAt: c.SyncedAt, SentAt: c.SentAt,
		SignalDate: c.SignalDate,
	})
}

func (s *Service) checkResult(c dbq.Ema1dDailyCheck, status, errMsg string) DailyCheckResult {
	return DailyCheckResult{Status: status, Date: c.SignalDate, RunID: c.RunID, Attempts: c.Attempts, Report: c.Report, Error: errMsg}
}

// reportParts splits on lines: Discord takes 2000 characters per message.
func reportParts(text string, limit int) []string {
	var parts []string
	var cur strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if cur.Len() > 0 && cur.Len()+len(line)+1 > limit {
			parts = append(parts, strings.TrimSpace(cur.String()))
			cur.Reset()
		}
		cur.WriteString(line)
		cur.WriteString("\n")
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		parts = append(parts, s)
	}
	return parts
}

// dailyCheckJob runs the check from the scheduler.
func (s *Service) dailyCheckJob(ctx context.Context) (string, error) {
	res, err := s.DailyCheck(ctx, DailyCheckOptions{})
	if err != nil {
		return "", err
	}
	switch res.Status {
	case "disabled":
		return "", scheduler.Skipf("daily check is disabled")
	case "already_sent":
		return "", scheduler.Skipf("already sent for %s", res.Date)
	case "failed":
		// Already alerted by the check itself: fail the run without a second alert.
		return "", scheduler.Quiet(errors.New(res.Error))
	}
	return fmt.Sprintf("%s %s", res.Date, res.Status), nil
}
