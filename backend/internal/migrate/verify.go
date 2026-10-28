package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"

	"bigwaves/internal/platform/db"
	"bigwaves/internal/platform/money"
)

// Check is one comparison between PostgreSQL and SQLite.
type Check struct {
	Name     string `json:"name"`
	Postgres string `json:"postgres"`
	SQLite   string `json:"sqlite"`
	OK       bool   `json:"ok"`
}

// Verify compares row counts, money totals and the open positions of both databases. The import
// is accepted only when every check is OK.
func Verify(ctx context.Context, pgURL, sqlitePath string) ([]Check, error) {
	pg, err := sql.Open("pgx", pgURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = pg.Close() }()
	lite, err := db.Open(ctx, sqlitePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = lite.Close() }()

	var out []Check
	add := func(name, a, b string) { out = append(out, Check{name, a, b, a == b}) }
	one := func(d *sql.DB, q string) string {
		var s sql.NullString
		if err := d.QueryRowContext(ctx, q).Scan(&s); err != nil {
			return "error: " + err.Error()
		}
		return s.String
	}
	// Money totals: summed in Go with decimals on both sides (never SUM() over TEXT in SQLite).
	sum := func(d *sql.DB, q string) string {
		rows, err := d.QueryContext(ctx, q)
		if err != nil {
			return "error: " + err.Error()
		}
		defer func() { _ = rows.Close() }()
		total := money.Zero
		for rows.Next() {
			var s sql.NullString
			if err := rows.Scan(&s); err != nil {
				return "error: " + err.Error()
			}
			if s.Valid {
				v, err := money.Parse(s.String)
				if err != nil {
					return "error: " + err.Error()
				}
				total = total.Add(v)
			}
		}
		if err := rows.Err(); err != nil {
			return "error: " + err.Error()
		}
		return total.String()
	}
	list := func(d *sql.DB, q string) string {
		rows, err := d.QueryContext(ctx, q)
		if err != nil {
			return "error: " + err.Error()
		}
		defer func() { _ = rows.Close() }()
		var items []string
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				return "error: " + err.Error()
			}
			items = append(items, s)
		}
		if err := rows.Err(); err != nil {
			return "error: " + err.Error()
		}
		slices.Sort(items)
		return strings.Join(items, ",")
	}

	counts := []struct{ name, pgQ, liteQ string }{
		{"users", `SELECT count(*)::text FROM users`, `SELECT count(*) FROM users`},
		{"coins", `SELECT count(*)::text FROM coins`, `SELECT count(*) FROM coins`},
		{"pairs (Binance)", `SELECT count(*)::text FROM pairs p JOIN markets m ON m.id = p.market_id WHERE upper(m.name) = 'BINANCE'`, `SELECT count(*) FROM pairs`},
		{"ema1d_pairs", `SELECT count(*)::text FROM ema1d_pairs`, `SELECT count(*) FROM ema1d_pairs`},
		{"ema1d positions", `SELECT count(*)::text FROM ema_trades WHERE source = 'EMA1D_EXIT'`, `SELECT count(*) FROM ema1d_positions`},
		{"ema1d_todos", `SELECT count(*)::text FROM ema1d_todos`, `SELECT count(*) FROM ema1d_todos`},
		{"ema1d_syncs", `SELECT count(*)::text FROM ema1d_syncs`, `SELECT count(*) FROM ema1d_syncs`},
		{"ema1d_candles", `SELECT count(*)::text FROM ema1d_candles`, `SELECT count(*) FROM ema1d_candles`},
		{"ema1d_equity_snapshots", `SELECT count(*)::text FROM ema1d_equity_snapshots`, `SELECT count(*) FROM ema1d_equity_snapshots`},
		{"ema1d_daily_checks", `SELECT count(*)::text FROM ema1d_daily_checks`, `SELECT count(*) FROM ema1d_daily_checks`},
		{"dca_settings", `SELECT count(*)::text FROM dca_settings`, `SELECT count(*) FROM dca_settings`},
		{"dca_trades", `SELECT count(*)::text FROM dca_trades`, `SELECT count(*) FROM dca_trades`},
		{"ema_settings", `SELECT count(*)::text FROM ema_settings`, `SELECT count(*) FROM ema_settings`},
		{"ema setting trades", `SELECT count(*)::text FROM ema_trades WHERE ema_setting_id IS NOT NULL`, `SELECT count(*) FROM ema_setting_trades`},
	}
	for _, c := range counts {
		add("count "+c.name, one(pg, c.pgQ), one(lite.Read, c.liteQ))
	}
	sums := []struct{ name, pgQ, liteQ string }{
		{"EMA1D realized profit", `SELECT realized_profit::text FROM ema_trades WHERE source = 'EMA1D_EXIT' AND status = 'DONE'`, `SELECT realized_profit FROM ema1d_positions WHERE status = 'CLOSED'`},
		{"EMA1D fees", `SELECT fee_usdt::text FROM ema_trades WHERE source = 'EMA1D_EXIT'`, `SELECT fee_usdt FROM ema1d_positions`},
		{"EMA1D interest", `SELECT interest_usdt::text FROM ema_trades WHERE source = 'EMA1D_EXIT'`, `SELECT interest_usdt FROM ema1d_positions`},
		{"EMA1D open amount", `SELECT amount::text FROM ema_trades WHERE source = 'EMA1D_EXIT' AND status IN ('OPENING', 'CLOSING')`, `SELECT amount FROM ema1d_positions WHERE status IN ('OPENING', 'OPEN')`},
		{"DCA realized profit", `SELECT realized_profit::text FROM dca_trades`, `SELECT realized_profit FROM dca_trades`},
		{"DCA held amount", `SELECT amount::text FROM dca_trades WHERE status = 'CLOSING'`, `SELECT held_amount FROM dca_trades WHERE status = 'OPEN'`},
		{"EMA Settings realized profit", `SELECT realized_profit::text FROM ema_trades WHERE ema_setting_id IS NOT NULL`, `SELECT realized_profit FROM ema_setting_trades`},
		{"EMA Settings budgets", `SELECT budget::text FROM ema_settings`, `SELECT budget FROM ema_settings`},
		{"EMA Settings open amount", `SELECT amount::text FROM ema_trades WHERE ema_setting_id IS NOT NULL AND status = 'CLOSING'`, `SELECT amount FROM ema_setting_trades WHERE status = 'OPEN'`},
	}
	for _, c := range sums {
		add("sum "+c.name, sum(pg, c.pgQ), sum(lite.Read, c.liteQ))
	}
	add("open EMA1D positions",
		list(pg, `SELECT p.name || ':' || upper(t.trade_type) || ':' || t.signal_date::text FROM ema_trades t JOIN pairs p ON p.id = t.pair_id WHERE t.source = 'EMA1D_EXIT' AND t.status IN ('OPENING', 'CLOSING')`),
		list(lite.Read, `SELECT p.name || ':' || x.side || ':' || x.signal_date FROM ema1d_positions x JOIN pairs p ON p.id = x.pair_id WHERE x.status IN ('OPENING', 'OPEN')`))
	add("open EMA Settings trades",
		list(pg, `SELECT t.id::text || ':' || p.name || ':' || upper(t.trade_type) || ':' || t.ema_setting_id::text FROM ema_trades t JOIN pairs p ON p.id = t.pair_id WHERE t.ema_setting_id IS NOT NULL AND t.status IN ('OPENING', 'CLOSING')`),
		list(lite.Read, `SELECT x.id || ':' || p.name || ':' || x.side || ':' || x.setting_id FROM ema_setting_trades x JOIN pairs p ON p.id = x.pair_id WHERE x.status IN ('OPENING', 'OPEN')`))
	add("ACTIVE EMA settings",
		list(pg, `SELECT id::text FROM ema_settings WHERE status = 'ACTIVE'`),
		list(lite.Read, `SELECT CAST(id AS TEXT) FROM ema_settings WHERE status = 'ACTIVE'`))
	add("in-flight todos",
		list(pg, `SELECT id::text || ':' || upper(status) FROM ema1d_todos WHERE status IN ('PENDING', 'RUNNING')`),
		list(lite.Read, `SELECT id || ':' || status FROM ema1d_todos WHERE status IN ('PENDING', 'RUNNING')`))
	add("candles identical (row by row)", "0 different", fmt.Sprintf("%d different", candleDiffs(ctx, pg, lite.Read)))
	return out, nil
}

// candleDiffs compares every candle in the same order on both sides, exactly (float64 equality).
func candleDiffs(ctx context.Context, pg, lite *sql.DB) int {
	q := `SELECT pair_id, interval, %s, open, high, low, close FROM ema1d_candles ORDER BY pair_id, interval, open_time`
	a, err := pg.QueryContext(ctx, fmt.Sprintf(q, "extract(epoch from open_time)::bigint"))
	if err != nil {
		return -1
	}
	defer func() { _ = a.Close() }()
	b, err := lite.QueryContext(ctx, fmt.Sprintf(q, "open_time"))
	if err != nil {
		return -1
	}
	defer func() { _ = b.Close() }()
	type row struct {
		pair       int64
		interval   string
		t          int64
		o, h, l, c float64
	}
	scan := func(r *sql.Rows) (row, bool) {
		var x row
		if !r.Next() || r.Scan(&x.pair, &x.interval, &x.t, &x.o, &x.h, &x.l, &x.c) != nil {
			return x, false
		}
		return x, true
	}
	diffs := 0
	for {
		x, okA := scan(a)
		y, okB := scan(b)
		if !okA && !okB {
			return diffs
		}
		if okA != okB || x != y {
			diffs++
		}
	}
}
