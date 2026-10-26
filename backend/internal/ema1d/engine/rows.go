package engine

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// PairInput describes one pair for the status rows.
type PairInput struct {
	Coin       string // "ETH"
	LongExit   string // DIV4H or XO
	LongOnly   bool   // never shorts the coin
	SignalFrom *int64 // unix seconds: entries before it are ignored (config.json "added")
	ExitOnly   bool   // taken out of the basket: only follows its open trade until it exits
	RemovedOn  *int64 // unix seconds (exit only)
}

// Row is one line of status.py, with the same column names in JSON. Open and Side are extra:
// whether the strategy holds a trade and which side ("L"/"S"), not part of the Python output.
type Row struct {
	Pair      string   `json:"pair"`
	Ngay      string   `json:"ngay"`
	Gia       float64  `json:"gia"`
	EmaOpen   string   `json:"ema_open"`
	CatHomNay string   `json:"cat_hom_nay"`
	Lenh      string   `json:"lenh"`
	Vao       *string  `json:"vao"`
	GiaVao    *float64 `json:"gia_vao"`
	Lai       string   `json:"lai"`
	ThoatTheo string   `json:"thoat_theo"`
	HanhDong  string   `json:"hanh_dong_hom_nay"`
	Nhom      string   `json:"nhom"`
	GhiChu    string   `json:"ghi_chu"`
	Open      bool     `json:"open"`
	Side      string   `json:"side,omitempty"`
}

// Values of Row.Lenh and Row.Nhom.
const (
	LenhLong    = "LONG"
	LenhShort   = "SHORT"
	LenhFlat    = "ĐỨNG NGOÀI"
	LenhExited  = "ĐÃ THOÁT XONG"
	NhomBasket  = "RỔ"
	NhomNew     = "MỚI"
	NhomOldBask = "RỔ CŨ"
)

// BuildRow computes where the strategy stands for one pair. d1 and h4 are the candles up to now,
// the running candle included.
func BuildRow(in PairInput, d1Rows, h4Rows []Candle) (Row, error) {
	d1 := NewSeries(d1Rows)
	var f4 *Series
	if len(h4Rows) > 0 {
		f4 = NewSeries(h4Rows)
	}
	if d1.Len() <= Warm {
		return Row{}, fmt.Errorf("%s: not enough 1D candles (%d)", in.Coin, d1.Len())
	}
	trades, err := PairTrades(d1, f4, in.LongExit, in.LongOnly)
	if err != nil {
		return Row{}, fmt.Errorf("%s: %w", in.Coin, err)
	}
	if in.ExitOnly {
		return exitOnlyRow(in, d1, trades), nil
	}
	return watchRow(in, d1, trades), nil
}

func strp(s string) *string { return &s }

func watchRow(in PairInput, d1 *Series, all []PairTrade) Row {
	var pre, t []PairTrade
	if in.SignalFrom != nil {
		for _, x := range all {
			if x.Entry < *in.SignalFrom {
				pre = append(pre, x)
			} else {
				t = append(t, x)
			}
		}
	} else {
		t = all
	}

	// In sequence: an entry while the previous trade is still open is skipped.
	var busy *int64
	var last *PairTrade
	for i := range t {
		x := t[i]
		if busy != nil && x.Entry < *busy {
			continue
		}
		et := x.ExitTime()
		busy = &et
		last = &t[i]
	}

	tday := d1.Time[len(d1.Time)-1]
	var acts []string
	for _, x := range t {
		if !x.Open && x.ExitTime() == tday && (busy == nil || x.Entry <= *busy) {
			acts = append(acts, fmt.Sprintf("ĐÓNG %s (%s) %s", sideName(x.Side), x.Rule, Pct1(x.Ret)))
		}
	}
	if last != nil && last.Entry == tday {
		acts = append(acts, fmt.Sprintf("VÀO %s @ %s", sideName(last.Side), G(last.EntryPx, 6)))
	}

	st := baseRow(in, d1)
	if last != nil && last.Open {
		px := last.EntryPx
		st.Lenh, st.Vao, st.GiaVao = sideName(last.Side), strp(iso(last.Entry)), &px
		st.Lai, st.ThoatTheo = Pct1(last.Ret), last.Rule
	} else {
		st.Lenh = LenhFlat
		if last != nil {
			st.Vao = strp(iso(last.Entry) + " → thoát " + iso(last.ExitTime()))
			st.Lai = "lệnh cuối " + Pct1(last.Ret)
			st.ThoatTheo = last.Rule
		}
	}
	st.HanhDong = "-"
	if len(acts) > 0 {
		st.HanhDong = strings.Join(acts, " · ")
	}
	st.Nhom = NhomBasket
	if in.SignalFrom != nil {
		st.Nhom = NhomNew
		note := "thêm vào rổ " + iso(*in.SignalFrom) + ", chỉ vào ở tín hiệu mới"
		if last == nil {
			note += " — chưa có tín hiệu mới"
			var po []PairTrade
			for _, x := range pre {
				if x.Open {
					po = append(po, x)
				}
			}
			if len(po) > 0 {
				p := po[len(po)-1]
				note += fmt.Sprintf(" (lệnh chiến lược %s từ %s là trước ngày thêm, không theo)", sideName(p.Side), iso(p.Entry))
			}
		}
		st.GhiChu = note
	}
	st.Open = last != nil && last.Open
	if last != nil {
		st.Side = last.Side
	}
	return st
}

// exitOnlyRow follows the trade a coin of the old basket held when it was removed, until that
// trade exits. It never enters.
func exitOnlyRow(in PairInput, d1 *Series, all []PairTrade) Row {
	t := all
	if in.SignalFrom != nil {
		t = nil
		for _, x := range all {
			if x.Entry >= *in.SignalFrom {
				t = append(t, x)
			}
		}
	}
	var x *PairTrade
	if in.RemovedOn != nil {
		var busy *int64
		for i := range t {
			y := t[i]
			if y.Entry >= *in.RemovedOn {
				continue
			}
			if busy != nil && y.Entry < *busy {
				continue
			}
			et := y.ExitTime()
			busy = &et
			x = &t[i]
		}
	}

	st := baseRow(in, d1)
	st.Nhom = NhomOldBask
	rtxt := "loại khỏi rổ ?"
	if in.RemovedOn != nil {
		rtxt = "loại khỏi rổ " + iso(*in.RemovedOn)
	}
	if x == nil {
		st.Lenh, st.HanhDong = LenhExited, "-"
		st.GhiChu = rtxt + "; không tìm thấy lệnh đang theo — không vào lại"
		return st
	}
	if x.Open {
		px := x.EntryPx
		st.Lenh, st.Vao, st.GiaVao, st.Lai, st.ThoatTheo = sideName(x.Side), strp(iso(x.Entry)), &px, Pct1(x.Ret), x.Rule
		st.HanhDong = "-"
		st.GhiChu = rtxt + "; chỉ chờ tín hiệu THOÁT (" + x.Rule + "), không vào lệnh mới"
		st.Open, st.Side = true, x.Side
		return st
	}
	ex := x.ExitTime()
	st.Lenh = LenhExited
	st.Vao = strp(iso(x.Entry) + " → thoát " + iso(ex))
	st.Lai, st.ThoatTheo = "lệnh cuối "+Pct1(x.Ret), x.Rule
	st.HanhDong = "-"
	if ex == d1.Time[len(d1.Time)-1] {
		st.HanhDong = fmt.Sprintf("ĐÓNG %s (%s) %s — rổ cũ, không vào lại", sideName(x.Side), x.Rule, Pct1(x.Ret))
	}
	st.GhiChu = rtxt + "; đã thoát xong " + iso(ex) + ", không vào lại (dọn ở lần tái chọn sau)"
	return st
}

func baseRow(in PairInput, d1 *Series) Row {
	n := len(d1.UpO)
	up, prev := d1.UpO[n-1], d1.UpO[n-2]
	r := Row{
		Pair: strings.ToUpper(in.Coin), Ngay: iso(d1.Time[len(d1.Time)-1]), Gia: d1.Close[len(d1.Close)-1],
		EmaOpen: "GIẢM", CatHomNay: "-",
	}
	if up {
		r.EmaOpen = "TĂNG"
	}
	switch {
	case up && !prev:
		r.CatHomNay = "LÊN"
	case !up && prev:
		r.CatHomNay = "XUỐNG"
	}
	return r
}

// SyncParams is the sync request built from the rows (build_sync.py).
type SyncParams struct {
	Date     string   `json:"date"`
	S        string   `json:"s"`
	Items    []string `json:"items"`
	Warnings []string `json:"warnings"`
}

// BuildSyncParams makes s = COIN:STATE:ENTRY_DATE:REF_PRICE:RULE joined by ",":
//
//	open trade (basket or exit only) → L|S, entry date, open of the entry candle, exit rule
//	flat / done                      → F, "-", open of today, rule of the last trade or "-"
//
// openOn returns the open price of coin on a "YYYY-MM-DD" day.
func BuildSyncParams(rows []Row, today string, openOn func(coin, day string) (float64, bool)) SyncParams {
	var warnings []string
	var days []string
	for _, r := range rows {
		if !slices.Contains(days, r.Ngay) {
			days = append(days, r.Ngay)
		}
	}
	slices.Sort(days)
	if len(days) != 1 {
		warnings = append(warnings, fmt.Sprintf("nhiều ngày nến trong status: %s", pyList(days)))
	}
	date := ""
	if len(days) > 0 {
		date = days[len(days)-1]
	}
	if date != today {
		warnings = append(warnings, fmt.Sprintf("ngày nến %s khác hôm nay UTC %s — dữ liệu chưa cập nhật?", date, today))
	}

	items := make([]string, 0, len(rows))
	for _, r := range rows {
		coin := strings.ToUpper(r.Pair)
		rule := "-"
		if r.ThoatTheo == RuleDIV4H || r.ThoatTheo == RuleXO {
			rule = r.ThoatTheo
		}
		if r.Lenh == LenhLong || r.Lenh == LenhShort {
			entry := ""
			if r.Vao != nil {
				entry = (*r.Vao)[:min(10, len(*r.Vao))]
			}
			px, ok := openOn(coin, entry)
			if !ok {
				warnings = append(warnings, fmt.Sprintf("%s: không có giá open ngày vào %s", coin, entry))
			}
			items = append(items, fmt.Sprintf("%s:%s:%s:%s:%s", coin, r.Lenh[:1], entry, fmtPrice(px, ok), rule))
		} else {
			px, ok := openOn(coin, date)
			if !ok {
				warnings = append(warnings, fmt.Sprintf("%s: không có giá open ngày %s", coin, date))
			}
			items = append(items, fmt.Sprintf("%s:F:-:%s:%s", coin, fmtPrice(px, ok), rule))
		}
	}
	return SyncParams{Date: date, S: strings.Join(items, ","), Items: items, Warnings: warnings}
}

func pyList(xs []string) string {
	q := make([]string, len(xs))
	for i, x := range xs {
		q[i] = `"` + x + `"`
	}
	return "[" + strings.Join(q, ", ") + "]"
}

func fmtPrice(px float64, ok bool) string {
	if !ok {
		return "-"
	}
	return G(px, 10)
}

func sideName(side string) string {
	if side == "L" {
		return LenhLong
	}
	return LenhShort
}

// ISO formats unix seconds as a UTC date.
func iso(sec int64) string { return time.Unix(sec, 0).UTC().Format("2006-01-02") }

// ISODate is exported for callers that need the same date format.
func ISODate(sec int64) string { return iso(sec) }
