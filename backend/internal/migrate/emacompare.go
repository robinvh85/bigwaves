package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"bigwaves/internal/emacross"
	"bigwaves/internal/exchange/binance"
)

// EMADiff is EMA14 / Smooth10 at one closed candle: what Rails stored in ema_trackings and what
// the Go scan computes from Binance klines.
type EMADiff struct {
	Pair, Period          string
	CandleTime            time.Time
	RailsEMA, GoEMA       float64
	RailsSmooth, GoSmooth float64
}

// Gap is (EMA14 - Smooth10) / Smooth10 in percent, the figure the entry and exit thresholds read.
func Gap(e, s float64) float64 { return (e - s) / s * 100 }

func (d EMADiff) String() string {
	return fmt.Sprintf("%-12s %s %s  EMA14 rails=%.8g go=%.8g (%+.4f%%)  gap rails=%+.4f%% go=%+.4f%%",
		d.Pair, d.Period, d.CandleTime.Format("2006-01-02 15:04"), d.RailsEMA, d.GoEMA, (d.GoEMA-d.RailsEMA)/d.RailsEMA*100,
		Gap(d.RailsEMA, d.RailsSmooth), Gap(d.GoEMA, d.GoSmooth))
}

// EMACompare reads the pairs of the ACTIVE Rails EMA settings and compares, on their last closed
// candles, the averages Rails tracked with the ones computed from klines (D-ES2: the Go scan has
// no ema_trackings). Read-only on both sides.
func EMACompare(ctx context.Context, pgURL string, bn *binance.Client, now time.Time, candles int) ([]EMADiff, error) {
	pg, err := sql.Open("pgx", pgURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = pg.Close() }()
	targets, err := emaTargets(ctx, pg)
	if err != nil {
		return nil, err
	}

	var out []EMADiff
	for _, t := range targets {
		interval, step := "1d", 24*time.Hour
		if t.period == "H4" {
			interval, step = "4h", 4*time.Hour
		}
		running := now.UTC().Truncate(step)
		ks, err := bn.Klines(ctx, t.name, interval, running.Add(-299*step), 300)
		if err != nil {
			return nil, fmt.Errorf("%s klines: %w", t.name, err)
		}
		closes := make([]float64, len(ks))
		at := map[int64]int{}
		for i, k := range ks {
			closes[i], _ = strconv.ParseFloat(k.Close.String(), 64)
			at[k.OpenTime.Unix()] = i
		}
		e, s := emacross.Lines(closes)
		tracked, err := emaTracked(ctx, pg, t, running.Add(-7*time.Hour), candles)
		if err != nil {
			return nil, err
		}
		for _, d := range tracked {
			if t.period == "D1" {
				// Rails keys a D1 row by its date at midnight Asia/Ho_Chi_Minh (17:00 UTC the day
				// before); the Binance candle of that date opens at 00:00 UTC. VN has no DST.
				d.CandleTime = d.CandleTime.Add(7 * time.Hour).Truncate(24 * time.Hour)
			}
			i, ok := at[d.CandleTime.Unix()]
			if !ok {
				continue
			}
			d.Pair, d.Period, d.GoEMA, d.GoSmooth = t.name, t.period, e[i], s[i]
			out = append(out, d)
		}
	}
	return out, nil
}

type emaTarget struct {
	id           int64
	name, period string
}

func emaTargets(ctx context.Context, pg *sql.DB) ([]emaTarget, error) {
	rows, err := pg.QueryContext(ctx, `SELECT DISTINCT p.id, p.name, upper(s.period_type) FROM ema_settings s JOIN pairs p ON p.id = s.pair_id
		WHERE s.status = 'ACTIVE' ORDER BY p.name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []emaTarget
	for rows.Next() {
		var t emaTarget
		if err := rows.Scan(&t.id, &t.name, &t.period); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// emaTracked returns the last ema_trackings rows of a pair before the running candle.
func emaTracked(ctx context.Context, pg *sql.DB, t emaTarget, before time.Time, limit int) ([]EMADiff, error) {
	rows, err := pg.QueryContext(ctx, `SELECT time_at, ema::float8, smooth_ema::float8 FROM ema_trackings
		WHERE pair_id = $1 AND upper(period_type) = $2 AND time_at < $3 ORDER BY time_at DESC LIMIT $4`, t.id, t.period, before, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []EMADiff
	for rows.Next() {
		var d EMADiff
		if err := rows.Scan(&d.CandleTime, &d.RailsEMA, &d.RailsSmooth); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
