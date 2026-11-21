package emacross

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"bigwaves/internal/exchange/binance"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/money"
	"bigwaves/internal/platform/notify"
	"bigwaves/internal/platform/scheduler"
)

const (
	// klineCount candles feed the averages: the seed no longer matters after a few dozen.
	klineCount = 300
	// minCandles below which a pair (newly listed) is not checked.
	minCandles = 3 * EMAPeriod
)

// Check reads every watched ACTIVE pair on D1 and sends each new cross to Discord. Errors of one
// pair are reported in the result and do not stop the others.
func (s *Service) Check(ctx context.Context) (string, error) {
	tf := D1
	cfg, _, err := s.Config(ctx)
	if err != nil {
		return "", err
	}
	if !cfg.Enabled {
		return "", scheduler.Skipf("EMA cross alerts are off")
	}
	pairs, err := s.watchedPairs(ctx)
	if err != nil {
		return "", err
	}
	if len(pairs) == 0 {
		return "", scheduler.Skipf("no pair watched")
	}
	threshold, _ := strconv.ParseFloat(cfg.ThresholdPercent.String(), 64)

	var crosses, sent int
	var errs []error
	for i, p := range pairs {
		if i > 0 {
			s.sleep(100 * time.Millisecond)
		}
		cross, isNew, err := s.checkPair(ctx, p, tf, threshold)
		if err != nil {
			// Account-wide problems fail every pair the same way: stop at the first one.
			if errors.Is(err, binance.ErrBanned) || ctx.Err() != nil {
				return "", err
			}
			errs = append(errs, fmt.Errorf("%s: %w", p.PairName, err))
			continue
		}
		if cross {
			crosses++
		}
		if isNew {
			sent++
		}
	}
	s.changed()
	msg := fmt.Sprintf("%s: %d pairs checked, %d crossed, %d alert(s) sent", tf.Name, len(pairs)-len(errs), crosses, sent)
	if len(errs) > 0 {
		return msg, fmt.Errorf("%s, %d failed: %w", msg, len(errs), errors.Join(errs...))
	}
	return msg, nil
}

func (s *Service) watchedPairs(ctx context.Context) ([]dbq.ListEmacrossPairsRow, error) {
	rows, err := dbq.New(s.DB.Read).ListEmacrossPairs(ctx)
	if err != nil {
		return nil, err
	}
	var out []dbq.ListEmacrossPairsRow
	for _, r := range rows {
		if r.PairStatus == "ACTIVE" {
			out = append(out, r)
		}
	}
	return out, nil
}

// checkPair stores the reading of one pair and sends its alert when the running candle crossed
// and has not been alerted yet.
func (s *Service) checkPair(ctx context.Context, p dbq.ListEmacrossPairsRow, tf Timeframe, threshold float64) (cross, sent bool, err error) {
	ks, err := s.klines(ctx, p.PairName, tf)
	if err != nil {
		return false, false, err
	}
	if len(ks) < minCandles {
		return false, false, fmt.Errorf("only %d %s candles on Binance, need %d", len(ks), tf.Name, minCandles)
	}
	closes := make([]float64, len(ks))
	for i, k := range ks {
		closes[i], _ = strconv.ParseFloat(k.Close.String(), 64)
	}
	r, _ := Evaluate(closes, threshold)
	last := ks[len(ks)-1]
	candle := last.OpenTime.Unix()

	q := dbq.New(s.DB.Write)
	if err := q.UpsertEmacrossState(ctx, dbq.UpsertEmacrossStateParams{
		PairID: p.EmacrossPair.PairID, Timeframe: tf.Name, CandleTime: candle, Trend: r.Trend,
		GapPercent: r.GapPercent, Price: last.Close, CheckedAt: dbtype.NewTime(s.now()),
	}); err != nil {
		return false, false, err
	}
	if r.Cross == "" {
		return false, false, nil
	}
	id, err := q.InsertEmacrossAlert(ctx, dbq.InsertEmacrossAlertParams{
		PairID: p.EmacrossPair.PairID, Timeframe: tf.Name, CandleTime: candle, Direction: r.Cross,
		Price: last.Close, GapPercent: r.GapPercent,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return true, false, nil // already sent for this candle
	}
	if err != nil {
		return true, false, err
	}
	sendErr := s.Notifier.Send(ctx, notify.Crossed, Message(tf, p.PairName, r.Cross, last.Close, r.GapPercent))
	fin := dbq.FinishEmacrossAlertParams{ID: id}
	if sendErr != nil {
		fin.Error = sendErr.Error()
	} else {
		now := dbtype.NewTime(s.now())
		fin.SentAt = &now
	}
	if err := q.FinishEmacrossAlert(ctx, fin); err != nil {
		return true, false, err
	}
	if sendErr != nil {
		return true, false, fmt.Errorf("send alert: %w", sendErr)
	}
	return true, true, nil
}

// Message keeps the Rails wording ("EMA D1: SUNUSDT is UP CROSSED") and adds the price and gap.
func Message(tf Timeframe, pair, direction string, price money.Dec, gapPercent float64) string {
	icon, side := "📈", "above"
	if direction == TrendDown {
		icon, side = "📉", "below"
	}
	return fmt.Sprintf("%s EMA %s: %s is %s CROSSED · price %s · EMA14 %s Smooth10 by %s%%",
		icon, tf.Name, pair, direction, price.String(), side,
		strings.TrimPrefix(strconv.FormatFloat(gapPercent, 'f', 2, 64), "-"))
}

// klines returns the last klineCount candles up to the running one, oldest first.
func (s *Service) klines(ctx context.Context, symbol string, tf Timeframe) ([]binance.Kline, error) {
	now := s.now()
	running := now.Truncate(tf.Step)
	start := running.Add(-time.Duration(klineCount-1) * tf.Step)
	ks, err := s.Binance.Klines(ctx, symbol, tf.API, start, klineCount)
	if err != nil {
		return nil, err
	}
	// Drop anything after the running candle (clock skew).
	for len(ks) > 0 && ks[len(ks)-1].OpenTime.After(running) {
		ks = ks[:len(ks)-1]
	}
	if len(ks) > 0 && ks[len(ks)-1].OpenTime.Before(running) {
		return nil, fmt.Errorf("no %s candle since %s UTC (trading halted?)", tf.Name, ks[len(ks)-1].OpenTime.Format("2006-01-02 15:04"))
	}
	return ks, nil
}
