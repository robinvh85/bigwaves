package migrate

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"bigwaves/internal/ema1d"
)

// ShadowResult compares what Rails and the bot decided for one candle date.
type ShadowResult struct {
	Date        string   `json:"date"`
	RailsRunID  string   `json:"railsRunId"`
	RailsS      string   `json:"railsS"`
	BotS        string   `json:"botS"`
	SameS       bool     `json:"sameS"`
	RailsTodos  []string `json:"railsTodos"`
	BotTodos    []string `json:"botTodos"`
	SameTodos   bool     `json:"sameTodos"`
	BotWarnings []string `json:"botWarnings"`
}

func (r ShadowResult) OK() bool { return r.SameS && r.SameTodos }

func (r ShadowResult) Text() string {
	status := "KHỚP"
	if !r.OK() {
		status = "LỆCH"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "**Shadow %s: %s**\n", r.Date, status)
	fmt.Fprintf(&b, "tín hiệu: %s\n", map[bool]string{true: "giống nhau", false: "KHÁC"}[r.SameS])
	if !r.SameS {
		fmt.Fprintf(&b, "  Rails: %s\n  Bot:   %s\n", r.RailsS, r.BotS)
	}
	fmt.Fprintf(&b, "todo: Rails [%s] · Bot [%s]\n", strings.Join(r.RailsTodos, ", "), strings.Join(r.BotTodos, ", "))
	if len(r.BotWarnings) > 0 {
		fmt.Fprintf(&b, "cảnh báo của bot: %s\n", strings.Join(r.BotWarnings, "; "))
	}
	return b.String()
}

// Shadow compares the Rails sync of date (its signal string and created todos) with what the bot
// computes from its own candles and a dry-run sync on the imported state. Run it right after the
// Rails daily check, on a database imported the night before.
func Shadow(ctx context.Context, pgURL string, s *ema1d.Service, date string) (ShadowResult, error) {
	res := ShadowResult{Date: date}
	pg, err := sql.Open("pgx", pgURL)
	if err != nil {
		return res, err
	}
	defer func() { _ = pg.Close() }()

	var runID, parsed, result string
	err = pg.QueryRowContext(ctx, `SELECT run_id, parsed::text, result::text FROM ema1d_syncs
		WHERE signal_date = $1 AND NOT dry_run ORDER BY created_at DESC LIMIT 1`, date).Scan(&runID, &parsed, &result)
	if errors.Is(err, sql.ErrNoRows) {
		return res, fmt.Errorf("rails has no real sync for %s yet", date)
	}
	if err != nil {
		return res, err
	}
	res.RailsRunID = runID
	var p struct {
		Items []ema1d.SyncItem `json:"items"`
	}
	_ = json.Unmarshal([]byte(parsed), &p)
	res.RailsS = itemsString(p.Items)
	var r ema1d.SyncResult
	_ = json.Unmarshal([]byte(result), &r)
	res.RailsTodos = todoKeys(r.CreatedTodos)

	if _, err := s.UpdateCandles(ctx); err != nil {
		return res, err
	}
	sig, err := s.Signals(ctx)
	if err != nil {
		return res, err
	}
	res.BotWarnings = sig.Warnings
	bot, err := s.Sync(ctx, ema1d.SyncRequest{Date: sig.Date, RunID: "shadow-" + strings.ReplaceAll(date, "-", ""), S: sig.S, DryRun: "1"}, "source=shadow")
	if err != nil {
		return res, err
	}
	res.BotS = normalizeS(sig.S)
	res.BotTodos = todoKeys(bot.CreatedTodos)
	res.SameS = res.RailsS == res.BotS
	res.SameTodos = slices.Equal(res.RailsTodos, res.BotTodos)
	return res, nil
}

func todoKeys(ts []ema1d.TodoSummary) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = fmt.Sprintf("%s %s %s %s", t.Action, t.Side, t.Coin, t.SignalDate)
	}
	slices.Sort(out)
	return out
}

// itemsString rebuilds the comparable part of S (coin, state, entry date, rule) from parsed items.
func itemsString(items []ema1d.SyncItem) string {
	parts := make([]string, len(items))
	for i, it := range items {
		parts[i] = fmt.Sprintf("%s:%s:%s:%s", it.Coin, it.State, orDash(it.EntryDate), orDash(it.Rule))
	}
	slices.Sort(parts)
	return strings.Join(parts, ",")
}

// normalizeS drops the reference prices (formatting may differ) and sorts by coin.
func normalizeS(s string) string {
	var parts []string
	for _, tok := range strings.Split(s, ",") {
		f := strings.Split(tok, ":")
		if len(f) < 5 {
			continue
		}
		parts = append(parts, strings.Join([]string{f[0], f[1], f[2], f[4]}, ":"))
	}
	slices.Sort(parts)
	return strings.Join(parts, ",")
}

func orDash(s *string) string {
	if s == nil || *s == "" {
		return "-"
	}
	return *s
}
