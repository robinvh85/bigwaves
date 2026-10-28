package migrate_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	"bigwaves/internal/migrate"
)

// The import test needs a PostgreSQL server: BIGWAVES_TEST_PG is an admin URL, e.g.
// "postgres:///postgres?host=/tmp". It creates a throwaway database with the Rails schema
// (testdata/rails_schema.sql, dumped from the Rails app) and the rows below.

const fixture = `
INSERT INTO markets (id, name, created_at, updated_at) VALUES (1, 'Binance', now(), now()), (2, 'OKX', now(), now());
INSERT INTO coins (id, name, description, created_at, updated_at) VALUES
  (1, 'SUN', '# SUN notes', now(), now()), (2, 'ETH', NULL, now(), now()), (3, 'XYZ', NULL, now(), now());
INSERT INTO pairs (id, coin_id, market_id, name, current_price, status, step_size, is_spot, is_isolated,
  is_auto_ema_h1_trade, is_auto_ema_h4_trade, is_auto_ema_d1_trade, created_at, updated_at) VALUES
  (1, 1, 1, 'SUNUSDT', 0.02, 'ACTIVE', 1, false, false, false, true, true, now(), now()),
  (2, 2, 1, 'ETHUSDT', 2000, 'ACTIVE', 0.0001, false, false, false, false, false, now(), now()),
  (3, 3, 2, 'XYZUSDT', 1, 'ACTIVE', 1, false, false, true, true, true, now(), now());
INSERT INTO users (id, email, uid, created_at, updated_at, ips) VALUES (1, 'Me@Example.com', 'auth0|1', now(), now(), '{1.2.3.4}');
INSERT INTO app_settings (key, value, created_at, updated_at) VALUES
  ('EMA1D_ENABLED', 1, now(), now()), ('EMA1D_BUDGET_USDT', 7000, now(), now()),
  ('EMA1D_SIZE_CORE_LONG', 5, now(), now()), ('EMA_H1_USD_AMOUNT_NEW_TRADE', 50, now(), now()),
  ('EMA_H4_OVER_CROSS_EMA_PERCENT_FOR_OPEN', 0.05, now(), now());
INSERT INTO ema1d_pairs (id, pair_id, pair_group, allow_long, allow_short, short_supported, status, added_on, long_exit_rule, created_at, updated_at) VALUES
  (1, 1, 'CORE', true, true, true, 'ACTIVE', '2026-10-01', 'DIV4H', now(), now()),
  (2, 2, 'CORE', true, true, true, 'ACTIVE', '2026-10-01', 'DIV4H', now(), now());
INSERT INTO ema_trades (id, pair_id, open_price, close_price, amount, status, trade_type, source, signal_date, exit_rule,
  size_percent, equity_at_open, notional_usdt, fee_usdt, realized_profit, unrealized_profit, open_at, close_at, created_at, updated_at) VALUES
  (10, 1, 0.02, 0, 10500, 'CLOSING', 'SHORT', 'EMA1D_EXIT', '2026-10-09', 'XO', 3.5, 6000, 210, 0.21, 0, 6.3, '2026-10-09 01:05:00', NULL, now(), now()),
  (11, 2, 2000, 2100, 0.2, 'DONE', 'LONG', 'EMA1D_EXIT', '2026-09-01', 'DIV4H', 7, 6000, 400, 0.8, 19.2, 0, '2026-09-01 00:10:00', '2026-09-20 00:10:00', now(), now()),
  (12, 2, 0, 0, 0.1, 'OPENING', 'LONG', 'EMA1D_EXIT', '2026-10-25', 'DIV4H', 3.5, 6000, 200, 0, 0, 0, NULL, NULL, now(), now()),
  (13, 1, 0.01, 0.03, 100, 'CLOSING', 'LONG', 'EMA_AUTO', NULL, NULL, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, now(), now());
INSERT INTO ema_settings (id, pair_id, budget, trade_type, period_type, check_type, next_trade_type, tp_percent, sl_percent, status, note, created_at, updated_at) VALUES
  (144, 1, 600, 'BOTH', 'D1', 'CHECK_AT_CLOSE', 'NEW_TRADE', 0, 0, 'ACTIVE', 'W: down', now(), now()),
  (90, 2, 500, 'ONLY_LONG', 'H4', 'CHECK_AT_OPEN', 'NOT_NEW_TRADE', 10, 5, 'DONE', NULL, now(), now());
INSERT INTO ema_trades (id, pair_id, open_price, close_price, sl_price, amount, status, trade_type, source, ema_setting_id, is_auto_trade,
  fee_usdt, realized_profit, unrealized_profit, open_at, close_at, created_at, updated_at) VALUES
  (2174, 1, 0.019, 0.015, 0.0206, 3141, 'CLOSING', 'SHORT', 'EMA_AUTO', 144, true, 0.6, 0, 7.2, '2026-10-17 04:45:11', NULL, now(), now()),
  (2175, 1, 0.018, 0, 0, 0, 'OPENING', 'LONG', 'EMA_AUTO', 144, false, 0, 0, 0, NULL, NULL, now(), now()),
  (2100, 2, 2000, 2100, 1840, 0.1, 'DONE', 'LONG', 'EMA_AUTO', 90, false, 0, 10, 0, '2026-08-01 00:00:00', '2026-08-05 00:00:00', now(), now());
INSERT INTO ema1d_syncs (id, run_id, signal_date, raw_query, parsed, result, dry_run, created_at, updated_at) VALUES
  (5, 'auto-20261009', '2026-10-09', 'date=2026-10-09&token=SECRET&run_id=auto', '{"items":[]}', '{"created_todos":[]}', false, now(), now());
INSERT INTO ema1d_todos (id, ema1d_pair_id, action, side, signal_date, ref_price, exit_rule, source, ema1d_sync_id, depends_on_id,
  ema_trade_id, status, attempts, run_after, last_error, step, first_attempted_at, created_at, updated_at) VALUES
  (20, 1, 'OPEN', 'SHORT', '2026-10-09', 0.0213, 'XO', 'SYNC', 5, NULL, 10, 'DONE', 1, NULL, NULL, 'BORROWED', '2026-10-09 01:00:00', now(), now()),
  (21, 1, 'CLOSE', 'SHORT', '2026-10-09', NULL, NULL, 'MANUAL', NULL, NULL, 10, 'PENDING', 3, '2026-10-26 01:00:00', 'timeout', NULL, '2026-10-26 00:00:00', now(), now()),
  (22, 2, 'OPEN', 'LONG', '2026-10-25', 2010, 'DIV4H', 'SYNC', 5, NULL, 12, 'RUNNING', 0, NULL, NULL, NULL, '2026-10-26 00:00:00', now(), now());
INSERT INTO ema1d_candles (pair_id, interval, open_time, open, high, low, close, created_at, updated_at) VALUES
  (1, '1D', '2026-10-24 00:00:00', 0.01674, 0.0171, 0.0163, 0.016712345678901234, now(), now());
INSERT INTO dca_settings (id, pair_id, status, used_budget, created_at, updated_at) VALUES (1, 1, 'ACTIVE', 0, now(), now());
INSERT INTO dca_trades (id, dca_setting_id, open_price, close_price, amount, realized_profit, unrealized_profit, status, open_at, close_at, created_at, updated_at) VALUES
  (30, 1, 0.015, 0.02, 1000, 0, 0, 'OPENING', NULL, NULL, now(), now()),
  (31, 1, 0.0158, 0.0185, 2000, 0, 1.2, 'CLOSING', '2026-10-20 00:00:00', NULL, now(), now()),
  (32, 1, 0.016, 0.018, 500, 1.0, 0, 'DONE', '2026-10-01 00:00:00', '2026-10-05 00:00:00', now(), now());
`

func setupPG(t *testing.T) string {
	admin := os.Getenv("BIGWAVES_TEST_PG")
	if admin == "" {
		t.Skip("BIGWAVES_TEST_PG not set")
	}
	ctx := context.Background()
	a, err := sql.Open("pgx", admin)
	require.NoError(t, err)
	t.Cleanup(func() { _ = a.Close() })
	name := fmt.Sprintf("bigwaves_import_%d", time.Now().UnixNano())
	_, err = a.ExecContext(ctx, "CREATE DATABASE "+name)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = a.ExecContext(context.Background(), "DROP DATABASE "+name+" WITH (FORCE)") })

	u, err := url.Parse(admin)
	require.NoError(t, err)
	u.Path = "/" + name
	pg, err := sql.Open("pgx", u.String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = pg.Close() })
	schema, err := os.ReadFile(filepath.Join("testdata", "rails_schema.sql"))
	require.NoError(t, err)
	_, err = pg.ExecContext(ctx, string(schema))
	require.NoError(t, err)
	_, err = pg.ExecContext(ctx, fixture)
	require.NoError(t, err)
	return u.String()
}

func TestImportPG(t *testing.T) {
	pgURL := setupPG(t)
	ctx := context.Background()
	dir := t.TempDir()
	out := filepath.Join(dir, "bigwaves.db")

	rep, err := migrate.ImportPG(ctx, pgURL, out, filepath.Join(dir, "archive"))
	require.NoError(t, err)
	require.Equal(t, 2, rep.Imported["pairs"], "the OKX pair is not imported")
	require.Equal(t, 3, rep.Imported["ema1d_positions"], "the legacy trade is not a position")
	require.Equal(t, 1, rep.Archived["ema_trades_legacy"])
	require.Contains(t, strings.Join(rep.Notes, "\n"), "EMA_H1_USD_AMOUNT_NEW_TRADE=50")
	require.Contains(t, strings.Join(rep.Notes, "\n"), "WARNING: EMA1D enabled was ON")

	checks, err := migrate.Verify(ctx, pgURL, out)
	require.NoError(t, err)
	for _, c := range checks {
		require.True(t, c.OK, "%s: pg=%s sqlite=%s", c.Name, c.Postgres, c.SQLite)
	}

	lite, err := sql.Open("sqlite", "file:"+out)
	require.NoError(t, err)
	defer func() { _ = lite.Close() }()
	row := func(q string, args ...any) []any {
		t.Helper()
		r, err := lite.QueryContext(ctx, q, args...)
		require.NoError(t, err)
		defer func() { _ = r.Close() }()
		cols, _ := r.Columns()
		require.True(t, r.Next(), q)
		require.NoError(t, r.Err())
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		require.NoError(t, r.Scan(ptrs...))
		return vals
	}

	// Positions keep their ids, map their status and keep the Rails client order ids.
	require.Equal(t, []any{"OPEN", "ema_10_open", "ema_10_close", "0.02", nil, "10500", "2026-10-09T01:05:00.000Z"},
		row(`SELECT status, open_order_id, close_order_id, open_price, close_price, amount, open_at FROM ema1d_positions WHERE id = 10`))
	require.Equal(t, []any{"CLOSED", "2100", "19.2", "0.8"},
		row(`SELECT status, close_price, realized_profit, fee_usdt FROM ema1d_positions WHERE id = 11`))
	require.Equal(t, []any{"OPENING", nil}, row(`SELECT status, open_price FROM ema1d_positions WHERE id = 12`))

	// Todos point at their positions; retry state is kept.
	require.Equal(t, []any{int64(10), "PENDING", int64(3), "timeout", "2026-10-26T01:00:00.000Z"},
		row(`SELECT position_id, status, attempts, last_error, run_after FROM ema1d_todos WHERE id = 21`))
	require.Equal(t, []any{int64(12), "RUNNING"}, row(`SELECT position_id, status FROM ema1d_todos WHERE id = 22`))
	require.Equal(t, []any{"BORROWED"}, row(`SELECT step FROM ema1d_todos WHERE id = 20`))

	// The API token in the logged query does not travel.
	require.Equal(t, []any{"date=2026-10-09&run_id=auto"}, row(`SELECT raw_query FROM ema1d_syncs WHERE id = 5`))

	// Config from the EMA1D settings.
	cfg := row(`SELECT json_extract(config, '$.enabled'), json_extract(config, '$.budgetUsdt'), json_extract(config, '$.sizes.coreLong') FROM strategy_configs WHERE strategy = 'ema1d'`)
	require.Equal(t, []any{int64(1), "7000", "5"}, cfg)

	// EMA cross: the watchlist from the Rails flags (not the OKX pair) and the H4 threshold.
	require.Equal(t, 1, rep.Imported["emacross_pairs"])
	require.Equal(t, []any{int64(1), int64(0), int64(1), int64(1)}, row(`SELECT pair_id, h1, h4, d1 FROM emacross_pairs`))
	require.Equal(t, []any{int64(1), "0.01", "0.05"},
		row(`SELECT json_extract(config, '$.enabled'), json_extract(config, '$.thresholdPercent.h1'), json_extract(config, '$.thresholdPercent.h4') FROM strategy_configs WHERE strategy = 'emacross'`))
	require.NotContains(t, strings.Join(rep.Notes, "\n"), "EMA_H4_OVER_CROSS_EMA_PERCENT_FOR_OPEN")

	// Floats survive exactly.
	require.Equal(t, []any{0.016712345678901234}, row(`SELECT close FROM ema1d_candles WHERE pair_id = 1`))

	// DCA: held rungs keep the fill as trigger (Rails overwrote it), costs are derived.
	require.Equal(t, []any{"OPENING", "0", nil, "0.015", "0.02"}, row(`SELECT status, held_amount, open_price, buy_price, sell_price FROM dca_trades WHERE id = 30`))
	require.Equal(t, []any{"OPEN", "2000", "0.0158", "31.6", "dca_31_close"}, row(`SELECT status, held_amount, open_price, buy_quote, close_order_id FROM dca_trades WHERE id = 31`))
	require.Equal(t, []any{"CLOSED", "0.018", "9", "1"}, row(`SELECT status, close_price, sell_quote, realized_profit FROM dca_trades WHERE id = 32`))

	// EMA Settings: settings and trades keep their ids and Rails order ids; open/close prices are
	// split into triggers and fills by status; the scan starts off with the Rails thresholds.
	require.Equal(t, 2, rep.Imported["ema_settings"])
	require.Equal(t, 3, rep.Imported["ema_setting_trades"])
	require.Equal(t, []any{"BOTH", "D1", "CHECK_AT_CLOSE", "NEW_TRADE", "600", "ACTIVE", "W: down"},
		row(`SELECT trade_type, period, check_type, next_trade_type, budget, status, note FROM ema_settings WHERE id = 144`))
	require.Equal(t, []any{"OPEN", "SHORT", "0.019", nil, "0.015", "0.0206", "7.2", "ema_2174_close", int64(1), int64(1)},
		row(`SELECT status, side, open_price, entry_price, target_price, sl_price, unrealized_profit, close_order_id, is_auto, legacy FROM ema_setting_trades WHERE id = 2174`))
	require.Equal(t, []any{"OPENING", "0.018", nil, nil}, row(`SELECT status, entry_price, open_price, target_price FROM ema_setting_trades WHERE id = 2175`))
	require.Equal(t, []any{"CLOSED", "2000", "2100", nil, "10"}, row(`SELECT status, open_price, close_price, target_price, realized_profit FROM ema_setting_trades WHERE id = 2100`))
	require.Equal(t, []any{int64(0), "0.05", "0.03"}, // d1 not in the fixture: default
		row(`SELECT json_extract(config, '$.enabled'), json_extract(config, '$.h4.openPercent'), json_extract(config, '$.d1.openPercent') FROM strategy_configs WHERE strategy = 'emasettings'`))
	require.Contains(t, strings.Join(rep.Notes, "\n"), "EMA Settings: 1 ACTIVE setting(s)")

	// Users are lower-cased; coin notes kept.
	require.Equal(t, []any{"me@example.com"}, row(`SELECT email FROM users`))
	require.Equal(t, []any{"# SUN notes"}, row(`SELECT description FROM coins WHERE id = 1`))

	_, err = os.Stat(filepath.Join(dir, "archive", "pairs.csv"))
	require.NoError(t, err)

	_, err = migrate.ImportPG(ctx, pgURL, out, "")
	require.ErrorContains(t, err, "already exists", "never overwrites an existing database")
}
