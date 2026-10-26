package ema1d

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"bigwaves/internal/ema1d/engine"
)

// reportText is the daily Discord report (same sections as before): actions today, the bot
// section, open LONG / SHORT, out of the market, old basket waiting to exit.
func reportText(date string, rows []engine.Row, bot []string, notes []string) string {
	var main, old []engine.Row
	var acts []string
	for _, r := range rows {
		if r.Nhom == engine.NhomOldBask {
			old = append(old, r)
		} else {
			main = append(main, r)
		}
		if r.HanhDong != "" && r.HanhDong != "-" {
			line := r.Pair + ": " + r.HanhDong
			if r.Nhom == engine.NhomOldBask {
				line += " (rổ cũ)"
			}
			acts = append(acts, line)
		}
	}
	long := openLines(main, engine.LenhLong)
	short := openLines(main, engine.LenhShort)
	var out []string
	for _, r := range main {
		if r.Lenh == engine.LenhLong || r.Lenh == engine.LenhShort {
			continue
		}
		lai := r.Lai
		if lai == "" {
			lai = "chưa có lệnh"
		}
		if r.Nhom == engine.NhomNew {
			lai += ", mới, chờ tín hiệu"
		}
		out = append(out, fmt.Sprintf("%s (%s)", r.Pair, lai))
	}

	d, _ := time.Parse("2006-01-02", date)
	lines := []string{fmt.Sprintf("**EMA 1D L+S — %s** (bigwaves)", d.Format("02/01/2006")), "", "**Hành động hôm nay**"}
	lines = append(lines, orDefault(acts, "không có tín hiệu mới")...)
	lines = append(lines, "", "**bigwaves**")
	lines = append(lines, bot...)
	lines = append(lines, "", fmt.Sprintf("**LONG đang mở (%d)**", len(long)))
	lines = append(lines, orDefault(long, "-")...)
	lines = append(lines, "", fmt.Sprintf("**SHORT đang mở (%d)**", len(short)))
	lines = append(lines, orDefault(short, "-")...)
	lines = append(lines, "", fmt.Sprintf("**Đứng ngoài (%d)**", len(out)))
	if len(out) > 0 {
		lines = append(lines, strings.Join(out, ", "))
	} else {
		lines = append(lines, "-")
	}
	if len(old) > 0 {
		lines = append(lines, "", "**Rổ cũ chờ thoát**")
		for _, r := range old {
			parts := []string{r.Pair, r.Lenh}
			if r.Vao != nil {
				parts = append(parts, *r.Vao)
			}
			for _, p := range []string{r.Lai, r.GhiChu} {
				if p != "" {
					parts = append(parts, p)
				}
			}
			lines = append(lines, strings.Join(parts, " "))
		}
	}
	if len(notes) > 0 {
		lines = append(lines, "", "**Dữ liệu**")
		lines = append(lines, notes...)
	}
	return strings.Join(lines, "\n")
}

func orDefault(xs []string, def string) []string {
	if len(xs) == 0 {
		return []string{def}
	}
	return xs
}

// openLines lists the open positions of one side, highest profit first.
func openLines(rows []engine.Row, side string) []string {
	var sel []engine.Row
	for _, r := range rows {
		if r.Lenh == side {
			sel = append(sel, r)
		}
	}
	pct := func(r engine.Row) float64 {
		v, _ := strconv.ParseFloat(strings.NewReplacer("%", "", "+", "").Replace(r.Lai), 64)
		return v
	}
	sort.SliceStable(sel, func(i, j int) bool { return pct(sel[i]) > pct(sel[j]) })
	out := make([]string, len(sel))
	for i, r := range sel {
		vao := ""
		if r.Vao != nil {
			if t, err := time.Parse("2006-01-02", *r.Vao); err == nil {
				vao = t.Format("02/01")
			}
		}
		px := ""
		if r.GiaVao != nil {
			px = engine.Repr(*r.GiaVao)
		}
		out[i] = fmt.Sprintf("%s %s (vào %s @%s)", r.Pair, r.Lai, vao, px)
	}
	return out
}

// botLines is the section about what the bot did with the signals: the sync result and the
// real positions.
func botLines(res SyncResult, st Status) []string {
	run := "sync " + res.RunID
	if res.Replayed {
		run += " (replayed)"
	}
	lines := []string{run}
	describe := func(ts []TodoSummary) string {
		parts := make([]string, len(ts))
		for i, t := range ts {
			parts[i] = fmt.Sprintf("%s %s %s", t.Action, t.Coin, t.Side)
		}
		return strings.Join(parts, ", ")
	}
	if len(res.CreatedTodos) > 0 {
		lines = append(lines, "TODO mới: "+describe(res.CreatedTodos))
	} else {
		lines = append(lines, "không có lệnh mới")
	}
	if len(res.CancelledTodos) > 0 {
		lines = append(lines, "TODO huỷ: "+describe(res.CancelledTodos))
	}
	if len(res.Skipped) > 0 {
		var order []string
		groups := map[string][]string{}
		for _, sk := range res.Skipped {
			reason, _, _ := strings.Cut(sk.Reason, " (")
			if _, ok := groups[reason]; !ok {
				order = append(order, reason)
			}
			groups[reason] = append(groups[reason], sk.Coin)
		}
		parts := make([]string, len(order))
		for i, reason := range order {
			parts[i] = reason + " " + strings.Join(groups[reason], "/")
		}
		lines = append(lines, fmt.Sprintf("bỏ qua %d: %s", len(res.Skipped), strings.Join(parts, "; ")))
	}
	if len(res.Warnings) > 0 {
		lines = append(lines, "cảnh báo: "+strings.Join(res.Warnings, "; "))
	}

	if len(st.OpenPositions) > 0 {
		items := make([]string, len(st.OpenPositions))
		for i, p := range st.OpenPositions {
			pnl := "?"
			if pct := percentOf(p.ProfitUsdt, deref(p.OpenPrice).Mul(p.Amount)); pct != nil {
				pnl = fmt.Sprintf("%+.2f%%", pct.InexactFloat64())
			}
			items[i] = fmt.Sprintf("%s %s %s %s", p.Coin, p.Side[:1], engine.G(deref(p.OpenPrice).InexactFloat64(), 6), pnl)
		}
		lines = append(lines, fmt.Sprintf("lệnh thật %d: %s", len(items), strings.Join(items, ", ")))
	} else {
		lines = append(lines, "lệnh thật: không có")
	}
	var failed []string
	for _, t := range st.Todos {
		if t.Status == TodoFailed {
			failed = append(failed, fmt.Sprintf("%s %s %s", t.Action, t.Coin, t.Side))
		}
	}
	if len(failed) > 0 {
		lines = append(lines, "TODO FAILED: "+strings.Join(failed, ", "))
	}
	if v := st.MarginLevel.Value; v != nil {
		line := fmt.Sprintf("margin level %s", v.Round(2))
		if st.MarginLevel.Warn.IsPositive() {
			line += fmt.Sprintf(" (warn %s)", st.MarginLevel.Warn)
			if v.LessThan(st.MarginLevel.Warn) {
				line += " — DƯỚI NGƯỠNG"
			}
		}
		lines = append(lines, line)
	}
	return lines
}
