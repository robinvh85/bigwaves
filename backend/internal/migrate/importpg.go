// Package migrate moves the Rails PostgreSQL data into a fresh SQLite database, checks the result,
// and compares the bot's decisions with the Rails app during the shadow run.
//
// Every value is read from PostgreSQL as text (numerics never go through float64) and ids are
// kept, so references between rows survive. Tables the rebuild dropped are exported as CSV.
package migrate

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver

	"bigwaves/internal/ema1d"
	"bigwaves/internal/emacross"
	"bigwaves/internal/platform/db"
	"bigwaves/internal/platform/money"
)

// Report says what the import did, table by table.
type Report struct {
	Imported map[string]int `json:"imported"`
	Archived map[string]int `json:"archived"`
	Skipped  []string       `json:"skipped"`
	Notes    []string       `json:"notes"`
}

// ts formats a Rails timestamp (stored UTC, without time zone) like dbtype.TimeLayout.
func ts(col string) string {
	return fmt.Sprintf(`to_char(%s, 'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"')`, col)
}

// imported lists the Rails tables this importer reads; everything else is archived.
var imported = []string{
	"users", "markets", "coins", "pairs", "app_settings", "ema1d_pairs", "ema_trades", "ema1d_syncs", "ema1d_todos",
	"ema1d_equity_snapshots", "ema1d_candles", "ema1d_daily_checks", "dca_settings", "dca_trades",
}

// noData are bookkeeping tables, neither imported nor archived.
var noData = []string{"ar_internal_metadata", "schema_migrations"}

// ImportPG copies the Rails data from pgURL into the SQLite file at sqlitePath, which must not
// exist yet. Dropped tables (and the EMA trades of the dropped strategies) go to archiveDir.
func ImportPG(ctx context.Context, pgURL, sqlitePath, archiveDir string) (Report, error) {
	rep := Report{Imported: map[string]int{}, Archived: map[string]int{}}
	if _, err := os.Stat(sqlitePath); err == nil {
		return rep, fmt.Errorf("%s already exists: import into a new file", sqlitePath)
	}
	pg, err := sql.Open("pgx", pgURL)
	if err != nil {
		return rep, err
	}
	defer func() { _ = pg.Close() }()
	if err := pg.PingContext(ctx); err != nil {
		return rep, fmt.Errorf("postgres: %w", err)
	}
	lite, err := db.Open(ctx, sqlitePath)
	if err != nil {
		return rep, err
	}
	defer func() { _ = lite.Close() }()
	if err := lite.Migrate(ctx); err != nil {
		return rep, err
	}

	im := &importer{pg: pg, lite: lite, rep: &rep}
	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"users", im.users}, {"coins", im.coins}, {"pairs", im.pairs}, {"config", im.config},
		{"ema1d_pairs", im.ema1dPairs}, {"ema1d_positions", im.positions}, {"ema1d_syncs", im.syncs},
		{"ema1d_todos", im.todos}, {"ema1d_equity_snapshots", im.snapshots}, {"ema1d_candles", im.candles},
		{"ema1d_daily_checks", im.dailyChecks}, {"dca", im.dca}, {"emacross_pairs", im.emaCrossPairs},
	}
	for _, st := range steps {
		if err := st.fn(ctx); err != nil {
			return rep, fmt.Errorf("import %s: %w", st.name, err)
		}
	}
	if archiveDir != "" {
		if err := im.archive(ctx, archiveDir); err != nil {
			return rep, fmt.Errorf("archive: %w", err)
		}
	}
	return rep, nil
}

type importer struct {
	pg   *sql.DB
	lite *db.DB
	rep  *Report
}

// copyRows runs query on PostgreSQL and inserts each row with insert, in one SQLite transaction.
// Every selected column must be text (or NULL).
func (im *importer) copyRows(ctx context.Context, table, query, insert string, transform func([]*string) []any) error {
	rows, err := im.pg.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	n := 0
	err = im.lite.Tx(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, insert)
		if err != nil {
			return err
		}
		defer func() { _ = stmt.Close() }()
		for rows.Next() {
			vals := make([]*string, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				return err
			}
			args := make([]any, len(vals))
			if transform != nil {
				args = transform(vals)
			} else {
				for i, v := range vals {
					args[i] = nullable(v)
				}
			}
			if _, err := stmt.ExecContext(ctx, args...); err != nil {
				return fmt.Errorf("row %v: %w", deref(vals[0]), err)
			}
			n++
		}
		return rows.Err()
	})
	im.rep.Imported[table] += n
	return err
}

func nullable(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// dec normalizes a PostgreSQL numeric ("12.50000000") to the canonical form ("12.5").
func dec(s *string) any {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if strings.Contains(v, ".") {
		v = strings.TrimRight(strings.TrimRight(v, "0"), ".")
	}
	if v == "" || v == "-0" {
		v = "0"
	}
	return v
}

func decOr0(s *string) any {
	if s == nil {
		return "0"
	}
	return dec(s)
}

// positive returns nil for zero or missing values (Rails used 0 for "not set yet").
func positive(s *string) any {
	v := dec(s)
	if v == nil || v == "0" {
		return nil
	}
	if f, err := strconv.ParseFloat(v.(string), 64); err == nil && f <= 0 {
		return nil
	}
	return v
}

func (im *importer) users(ctx context.Context) error {
	return im.copyRows(ctx, "users",
		`SELECT id::text, lower(email), uid, `+ts("created_at")+`, `+ts("updated_at")+` FROM users ORDER BY id`,
		`INSERT INTO users (id, email, uid, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, nil)
}

func (im *importer) coins(ctx context.Context) error {
	return im.copyRows(ctx, "coins",
		`SELECT id::text, name, coalesce(description, ''), `+ts("created_at")+`, `+ts("updated_at")+` FROM coins ORDER BY id`,
		`INSERT INTO coins (id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, nil)
}

var quotes = []string{"USDT", "FDUSD", "USDC", "BTC", "ETH", "BNB"}

func quoteOf(name string) string {
	for _, q := range quotes {
		if strings.HasSuffix(name, q) && len(name) > len(q) {
			return q
		}
	}
	return "USDT"
}

// pairs imports the Binance pairs. Pairs of other markets (OKX, MEXC… no longer traded) are
// archived with the rest.
func (im *importer) pairs(ctx context.Context) error {
	var others int
	_ = im.pg.QueryRowContext(ctx, `SELECT count(*) FROM pairs p JOIN markets m ON m.id = p.market_id WHERE upper(m.name) <> 'BINANCE'`).Scan(&others)
	if others > 0 {
		im.rep.Notes = append(im.rep.Notes, fmt.Sprintf("%d pair(s) of other markets not imported (see archive/pairs.csv)", others))
	}
	return im.copyRows(ctx, "pairs",
		`SELECT p.id::text, p.coin_id::text, p.name, p.status, p.current_price::text, `+ts("p.updated_at")+`,
		        p.interest_rate::text, p.step_size::text, p.is_spot::text, p.is_isolated::text,
		        `+ts("p.deleted_at")+`, `+ts("p.created_at")+`, `+ts("p.updated_at")+`
		 FROM pairs p JOIN markets m ON m.id = p.market_id WHERE upper(m.name) = 'BINANCE' ORDER BY p.id`,
		`INSERT INTO pairs (id, coin_id, market_id, name, quote_asset, status, current_price, price_updated_at, interest_rate,
		   step_size, is_spot, is_isolated, deleted_at, created_at, updated_at)
		 VALUES (?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		func(v []*string) []any {
			status := strings.ToUpper(deref(v[3]))
			if status != "ACTIVE" {
				status = "DISABLED"
			}
			return []any{
				*v[0], *v[1], *v[2], quoteOf(*v[2]), status, positive(v[4]), nullable(v[5]), dec(v[6]), positive(v[7]),
				pgBool(v[8]), pgBool(v[9]), nullable(v[10]), *v[11], *v[12],
			}
		})
}

// emaCrossConfig carries the cross thresholds (EMA_*_OVER_CROSS_EMA_PERCENT_FOR_OPEN) over to the
// EMA cross alert config.
func (im *importer) emaCrossConfig(ctx context.Context, get func(string) (string, bool)) error {
	c := emacross.DefaultConfig()
	for _, t := range []struct {
		key string
		dst *money.Dec
	}{
		{"EMA_H1_OVER_CROSS_EMA_PERCENT_FOR_OPEN", &c.ThresholdPercent.H1},
		{"EMA_H4_OVER_CROSS_EMA_PERCENT_FOR_OPEN", &c.ThresholdPercent.H4},
		{"EMA_D1_OVER_CROSS_EMA_PERCENT_FOR_OPEN", &c.ThresholdPercent.D1},
	} {
		if v, ok := get(t.key); ok {
			d, err := money.Parse(v)
			if err != nil {
				return fmt.Errorf("%s: %w", t.key, err)
			}
			*t.dst = d
		}
	}
	if err := c.Validate(); err != nil {
		return fmt.Errorf("EMA cross settings: %w", err)
	}
	raw, _ := json.Marshal(c)
	if _, err := im.lite.Write.ExecContext(ctx, `INSERT INTO strategy_configs (strategy, config) VALUES (?, ?)`, emacross.StrategyKey, string(raw)); err != nil {
		return err
	}
	im.rep.Imported["strategy_configs"]++
	return nil
}

// emaCrossPairs builds the EMA cross watchlist from the Rails flags pairs.is_auto_ema_*_trade.
func (im *importer) emaCrossPairs(ctx context.Context) error {
	err := im.copyRows(ctx, "emacross_pairs",
		`SELECT p.id::text, p.is_auto_ema_h1_trade::text, p.is_auto_ema_h4_trade::text, p.is_auto_ema_d1_trade::text
		 FROM pairs p JOIN markets m ON m.id = p.market_id
		 WHERE upper(m.name) = 'BINANCE' AND p.deleted_at IS NULL
		   AND (p.is_auto_ema_h1_trade OR p.is_auto_ema_h4_trade OR p.is_auto_ema_d1_trade)
		 ORDER BY p.id`,
		`INSERT INTO emacross_pairs (pair_id, h1, h4, d1) VALUES (?, ?, ?, ?)`,
		func(v []*string) []any { return []any{*v[0], pgBool(v[1]), pgBool(v[2]), pgBool(v[3])} })
	if err == nil && im.rep.Imported["emacross_pairs"] > 0 {
		im.rep.Notes = append(im.rep.Notes, fmt.Sprintf("EMA cross alerts watch %d pair(s): leave DISCORD_CROSS_WEBHOOK empty while Rails still sends them", im.rep.Imported["emacross_pairs"]))
	}
	return err
}

func pgBool(s *string) int {
	if s != nil && (*s == "true" || *s == "t") {
		return 1
	}
	return 0
}

// config builds the EMA1D config from the EMA1D_* AppSettings; the other ~60 keys belonged to
// dropped features and are listed in the report with their values.
func (im *importer) config(ctx context.Context) error {
	rows, err := im.pg.QueryContext(ctx, `SELECT key, value::text FROM app_settings ORDER BY key`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	settings := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return err
		}
		settings[k] = dec(&v).(string)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	c := ema1d.DefaultConfig()
	used := map[string]bool{}
	get := func(key string) (string, bool) {
		v, ok := settings[key]
		used[key] = ok
		return v, ok
	}
	setDec := func(key string, dst *string) {
		if v, ok := get(key); ok {
			*dst = v
		}
	}
	var budget, coreL, coreS, sensL, sensS, warn, alert string
	for _, p := range []struct {
		k string
		d *string
	}{
		{"EMA1D_BUDGET_USDT", &budget}, {"EMA1D_SIZE_CORE_LONG", &coreL}, {"EMA1D_SIZE_CORE_SHORT", &coreS},
		{"EMA1D_SIZE_SENSITIVE_LONG", &sensL}, {"EMA1D_SIZE_SENSITIVE_SHORT", &sensS},
		{"EMA1D_MARGIN_LEVEL_WARN", &warn}, {"EMA1D_MARGIN_LEVEL_ALERT", &alert},
	} {
		setDec(p.k, p.d)
	}
	b := func(key string) bool { v, ok := get(key); return ok && v != "0" }
	c.Enabled, c.PauseOpen, c.DailyCheckEnabled = b("EMA1D_ENABLED"), b("EMA1D_PAUSE_OPEN"), b("EMA1D_DAILY_CHECK_ENABLED")
	if v, ok := get("EMA1D_MAX_ENTRY_AGE_DAYS"); ok {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			c.MaxEntryAgeDays = int(n)
		}
	}
	raw, _ := json.Marshal(c)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	sizes, _ := m["sizes"].(map[string]any)
	for k, v := range map[string]string{"budgetUsdt": budget, "marginLevelWarn": warn, "marginLevelAlert": alert} {
		if v != "" {
			m[k] = v
		}
	}
	for k, v := range map[string]string{"coreLong": coreL, "coreShort": coreS, "sensitiveLong": sensL, "sensitiveShort": sensS} {
		if v != "" {
			sizes[k] = v
		}
	}
	raw, _ = json.Marshal(m)
	var check ema1d.Config
	if err := json.Unmarshal(raw, &check); err != nil {
		return err
	}
	if err := check.Validate(); err != nil {
		return fmt.Errorf("EMA1D settings: %w", err)
	}
	if _, err := im.lite.Write.ExecContext(ctx, `INSERT INTO strategy_configs (strategy, config) VALUES (?, ?)`, ema1d.StrategyKey, string(raw)); err != nil {
		return err
	}
	im.rep.Imported["strategy_configs"]++
	if err := im.emaCrossConfig(ctx, get); err != nil {
		return err
	}
	var dropped []string
	for k, v := range settings {
		if !used[k] {
			dropped = append(dropped, k+"="+v)
		}
	}
	slices.Sort(dropped)
	if len(dropped) > 0 {
		im.rep.Notes = append(im.rep.Notes, "app settings not carried over: "+strings.Join(dropped, ", "))
	}
	if check.Enabled {
		im.rep.Notes = append(im.rep.Notes, "WARNING: EMA1D enabled was ON in Rails: it is ON here too; keep TRADING_ENABLED=false until cutover")
	}
	return nil
}

func (im *importer) ema1dPairs(ctx context.Context) error {
	return im.copyRows(ctx, "ema1d_pairs",
		`SELECT id::text, pair_id::text, upper(pair_group), allow_long::text, allow_short::text, short_supported::text,
		        upper(status), added_on::text, removed_on::text, long_exit_rule, signal_from::text, `+ts("checked_at")+`,
		        coalesce(check_note, ''), `+ts("created_at")+`, `+ts("updated_at")+`
		 FROM ema1d_pairs ORDER BY id`,
		`INSERT INTO ema1d_pairs (id, pair_id, pair_group, allow_long, allow_short, short_supported, status, added_on,
		   removed_on, long_exit_rule, signal_from, checked_at, check_note, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		func(v []*string) []any {
			return []any{*v[0], *v[1], *v[2], pgBool(v[3]), pgBool(v[4]), pgBool(v[5]), *v[6], *v[7], nullable(v[8]),
				nullable(v[9]), nullable(v[10]), nullable(v[11]), *v[12], *v[13], *v[14]}
		})
}

var positionStatus = map[string]string{"OPENING": "OPENING", "CLOSING": "OPEN", "DONE": "CLOSED"}

// positions imports the EMA1D trades (source EMA1D_EXIT) as positions with the same ids, and keeps
// their Rails client order ids so an order placed by Rails is found, not placed again.
func (im *importer) positions(ctx context.Context) error {
	return im.copyRows(ctx, "ema1d_positions",
		`SELECT t.id::text, e.id::text, t.pair_id::text, upper(t.trade_type), upper(t.status), t.signal_date::text,
		        t.exit_rule, t.amount::text, t.open_price::text, t.close_price::text, t.ref_open_price::text,
		        t.ref_close_price::text, t.size_percent::text, t.equity_at_open::text, t.notional_usdt::text, t.fee_usdt::text,
		        t.interest_paid::text, t.interest_usdt::text, t.realized_profit::text, t.unrealized_profit::text,
		        `+ts("t.open_at")+`, `+ts("t.close_at")+`, `+ts("t.created_at")+`, `+ts("t.updated_at")+`
		 FROM ema_trades t JOIN ema1d_pairs e ON e.pair_id = t.pair_id
		 WHERE t.source = 'EMA1D_EXIT' ORDER BY t.id`,
		`INSERT INTO ema1d_positions (id, ema1d_pair_id, pair_id, side, status, signal_date, exit_rule, amount, open_price,
		   close_price, ref_open_price, ref_close_price, size_percent, equity_at_open, notional_usdt, fee_usdt, interest_paid,
		   interest_usdt, realized_profit, unrealized_profit, open_order_id, close_order_id, legacy_trade_id, open_at,
		   close_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		func(v []*string) []any {
			id := *v[0]
			status := positionStatus[*v[4]]
			closePrice := any(nil)
			if status == "CLOSED" {
				closePrice = positive(v[9])
			}
			signal := deref(v[5])
			if signal == "" && v[22] != nil {
				signal = (*v[22])[:10]
			}
			return []any{
				id, *v[1], *v[2], *v[3], status, signal, nullable(v[6]), decOr0(v[7]), positive(v[8]), closePrice,
				positive(v[10]), positive(v[11]), decOr0(v[12]), decOr0(v[13]), decOr0(v[14]), decOr0(v[15]), decOr0(v[16]),
				decOr0(v[17]), decOr0(v[18]), decOr0(v[19]), "ema_" + id + "_open", "ema_" + id + "_close", id,
				nullable(v[20]), nullable(v[21]), *v[22], *v[23],
			}
		})
}

func (im *importer) syncs(ctx context.Context) error {
	return im.copyRows(ctx, "ema1d_syncs",
		`SELECT id::text, run_id, signal_date::text, coalesce(raw_query, ''), parsed::text, result::text, dry_run::text, `+ts("created_at")+`
		 FROM ema1d_syncs ORDER BY id`,
		`INSERT INTO ema1d_syncs (id, run_id, signal_date, raw_query, parsed, result, dry_run, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		func(v []*string) []any {
			// A token in the logged query must not travel to the new database.
			return []any{*v[0], *v[1], *v[2], redactToken(*v[3]), *v[4], *v[5], pgBool(v[6]), *v[7]}
		})
}

func redactToken(q string) string {
	parts := strings.Split(q, "&")
	out := parts[:0]
	for _, p := range parts {
		if !strings.HasPrefix(p, "token=") {
			out = append(out, p)
		}
	}
	return strings.Join(out, "&")
}

func (im *importer) todos(ctx context.Context) error {
	// Todos pointing at a non-EMA1D trade cannot exist (EMA1D creates its own trades); drop the
	// reference if one does rather than fail the whole import.
	return im.copyRows(ctx, "ema1d_todos",
		`SELECT t.id::text, t.ema1d_pair_id::text, upper(t.action), upper(t.side), t.signal_date::text, t.ref_date::text,
		        t.ref_price::text, t.exit_rule, upper(t.source), t.ema1d_sync_id::text, t.depends_on_id::text,
		        (CASE WHEN tr.source = 'EMA1D_EXIT' THEN t.ema_trade_id END)::text, upper(t.status), t.attempts::text,
		        `+ts("t.first_attempted_at")+`, `+ts("t.run_after")+`, coalesce(t.last_error, ''), t.step,
		        `+ts("t.created_at")+`, `+ts("t.updated_at")+`
		 FROM ema1d_todos t LEFT JOIN ema_trades tr ON tr.id = t.ema_trade_id
		 ORDER BY t.id`,
		`INSERT INTO ema1d_todos (id, ema1d_pair_id, action, side, signal_date, ref_date, ref_price, exit_rule, source,
		   ema1d_sync_id, depends_on_id, position_id, status, attempts, first_attempted_at, run_after, last_error, step,
		   created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		func(v []*string) []any {
			out := make([]any, len(v))
			for i, x := range v {
				out[i] = nullable(x)
			}
			out[6] = positive(v[6])
			return out
		})
}

func (im *importer) snapshots(ctx context.Context) error {
	return im.copyRows(ctx, "ema1d_equity_snapshots",
		`SELECT date::text, equity_usdt::text, margin_level::text, open_long_notional::text, open_short_notional::text, `+ts("created_at")+`
		 FROM ema1d_equity_snapshots ORDER BY date`,
		`INSERT INTO ema1d_equity_snapshots (date, equity_usdt, margin_level, open_long_notional, open_short_notional, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		func(v []*string) []any {
			return []any{*v[0], decOr0(v[1]), positive(v[2]), decOr0(v[3]), decOr0(v[4]), *v[5]}
		})
}

// candles keeps the float values exactly: PostgreSQL float8::text prints the shortest round-trip
// form, which strconv parses back to the same float64.
func (im *importer) candles(ctx context.Context) error {
	return im.copyRows(ctx, "ema1d_candles",
		`SELECT pair_id::text, interval, extract(epoch from open_time)::bigint::text, open::text, high::text, low::text, close::text
		 FROM ema1d_candles ORDER BY pair_id, interval, open_time`,
		`INSERT INTO ema1d_candles (pair_id, interval, open_time, open, high, low, close) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		func(v []*string) []any {
			out := []any{*v[0], *v[1], *v[2]}
			for _, s := range v[3:] {
				f, _ := strconv.ParseFloat(*s, 64)
				out = append(out, f)
			}
			return out
		})
}

func (im *importer) dailyChecks(ctx context.Context) error {
	return im.copyRows(ctx, "ema1d_daily_checks",
		`SELECT signal_date::text, upper(status), coalesce(run_id, ''), attempts::text, rows::text, sync_params::text,
		        sync_result::text, coalesce(report, ''), coalesce(last_error, ''), `+ts("synced_at")+`, `+ts("sent_at")+`,
		        `+ts("created_at")+`, `+ts("updated_at")+`
		 FROM ema1d_daily_checks ORDER BY signal_date`,
		`INSERT INTO ema1d_daily_checks (signal_date, status, run_id, attempts, rows, sync_params, sync_result, report,
		   last_error, synced_at, sent_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, nil)
}

var dcaStatus = map[string]string{"OPENING": "OPENING", "CLOSING": "OPEN", "DONE": "CLOSED"}

// dca imports ladders and rungs. Rails overwrote open_price with the fill once bought, so for
// held and closed rungs the buy trigger is the fill price.
func (im *importer) dca(ctx context.Context) error {
	if err := im.copyRows(ctx, "dca_settings",
		`SELECT id::text, pair_id::text, upper(status), `+ts("created_at")+`, `+ts("updated_at")+` FROM dca_settings ORDER BY id`,
		`INSERT INTO dca_settings (id, pair_id, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, nil); err != nil {
		return err
	}
	return im.copyRows(ctx, "dca_trades",
		`SELECT id::text, dca_setting_id::text, upper(status), open_price::text, close_price::text, amount::text,
		        realized_profit::text, unrealized_profit::text, `+ts("open_at")+`, `+ts("close_at")+`,
		        `+ts("created_at")+`, `+ts("updated_at")+`,
		        (open_price * amount)::text, (close_price * amount)::text
		 FROM dca_trades ORDER BY id`,
		`INSERT INTO dca_trades (id, dca_setting_id, status, buy_price, sell_price, amount, held_amount, open_price,
		   close_price, buy_quote, sell_quote, realized_profit, unrealized_profit, open_order_id, close_order_id,
		   legacy_trade_id, open_at, close_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		func(v []*string) []any {
			id, status := *v[0], dcaStatus[*v[2]]
			held, openFill, closeFill, buyQuote, sellQuote := any("0"), any(nil), any(nil), any("0"), any("0")
			if status != "OPENING" {
				held, openFill, buyQuote = decOr0(v[5]), positive(v[3]), decOr0(v[12])
			}
			if status == "CLOSED" {
				closeFill, sellQuote = positive(v[4]), decOr0(v[13])
			}
			return []any{
				id, *v[1], status, decOr0(v[3]), decOr0(v[4]), decOr0(v[5]), held, openFill, closeFill, buyQuote, sellQuote,
				decOr0(v[6]), decOr0(v[7]), "dca_" + id + "_open", "dca_" + id + "_close", id, nullable(v[8]), nullable(v[9]),
				*v[10], *v[11],
			}
		})
}

// archive writes every table the rebuild dropped to dir/<table>.csv, plus the EMA trades of the
// dropped strategies and the pairs of other markets.
func (im *importer) archive(ctx context.Context, dir string) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tables, err := im.tables(ctx)
	if err != nil {
		return err
	}
	for _, t := range tables {
		switch {
		case slices.Contains(noData, t):
			continue
		case t == "ema_trades":
			if err := im.exportCSV(ctx, dir, "ema_trades_legacy", `SELECT * FROM ema_trades WHERE source <> 'EMA1D_EXIT' ORDER BY id`); err != nil {
				return err
			}
		case t == "pairs":
			if err := im.exportCSV(ctx, dir, "pairs", `SELECT * FROM pairs ORDER BY id`); err != nil {
				return err
			}
		case t == "app_settings" || t == "markets":
			if err := im.exportCSV(ctx, dir, t, `SELECT * FROM `+t+` ORDER BY id`); err != nil {
				return err
			}
		case slices.Contains(imported, t):
			continue
		default:
			if err := im.exportCSV(ctx, dir, t, `SELECT * FROM "`+t+`"`); err != nil {
				return err
			}
		}
	}
	return nil
}

func (im *importer) tables(ctx context.Context) ([]string, error) {
	rows, err := im.pg.QueryContext(ctx, `SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

func (im *importer) exportCSV(ctx context.Context, dir, name, query string) error {
	rows, err := im.pg.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, name+".csv")) //nolint:gosec // operator-provided directory
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	_ = w.Write(cols)
	n := 0
	for rows.Next() {
		vals := make([]sql.NullString, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			// Columns that do not scan to text (arrays): fall back to PostgreSQL's own CSV output.
			_ = f.Close()
			return im.exportViaText(ctx, dir, name, query)
		}
		rec := make([]string, len(cols))
		for i, v := range vals {
			rec[i] = v.String
		}
		_ = w.Write(rec)
		n++
	}
	w.Flush()
	if err := errors.Join(rows.Err(), w.Error(), f.Close()); err != nil {
		return err
	}
	im.rep.Archived[name] = n
	return nil
}

// exportViaText casts every column to text in SQL.
func (im *importer) exportViaText(ctx context.Context, dir, name, query string) error {
	// query is built by archive from table names listed by pg_tables, never from user input.
	q := `SELECT row_to_json(x)::text FROM (` + query + `) x` //nolint:gosec // see above
	rows, err := im.pg.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	f, err := os.Create(filepath.Join(dir, name+".jsonl")) //nolint:gosec // operator-provided directory
	if err != nil {
		return err
	}
	n := 0
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			_ = f.Close()
			return err
		}
		_, _ = io.WriteString(f, line+"\n")
		n++
	}
	im.rep.Archived[name] = n
	return errors.Join(rows.Err(), f.Close())
}
