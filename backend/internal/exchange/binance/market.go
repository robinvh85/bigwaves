package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"bigwaves/internal/platform/money"
)

// Kline is one candle. Prices are kept as decimals; indicator code converts to float64 itself.
type Kline struct {
	OpenTime  time.Time
	CloseTime time.Time
	Open      money.Dec
	High      money.Dec
	Low       money.Dec
	Close     money.Dec
	Volume    money.Dec
}

// Klines returns up to limit candles (max 1000) for symbol at interval ("1d", "4h"), starting at
// start when it is not zero.
func (c *Client) Klines(ctx context.Context, symbol, interval string, start time.Time, limit int) ([]Kline, error) {
	p := url.Values{"symbol": {symbol}, "interval": {interval}}
	if !start.IsZero() {
		p.Set("startTime", strconv.FormatInt(start.UnixMilli(), 10))
	}
	if limit > 0 {
		p.Set("limit", strconv.Itoa(limit))
	}
	var raw [][]json.RawMessage
	if err := c.do(ctx, request{method: http.MethodGet, path: "/api/v3/klines", params: p}, &raw); err != nil {
		return nil, err
	}
	out := make([]Kline, 0, len(raw))
	for _, row := range raw {
		k, err := parseKline(row)
		if err != nil {
			return nil, fmt.Errorf("kline %s %s: %w", symbol, interval, err)
		}
		out = append(out, k)
	}
	return out, nil
}

func parseKline(row []json.RawMessage) (Kline, error) {
	if len(row) < 7 {
		return Kline{}, fmt.Errorf("short row (%d fields)", len(row))
	}
	var openMs, closeMs int64
	var o, h, l, cl, v string
	for i, dst := range []any{&openMs, &o, &h, &l, &cl, &v, &closeMs} {
		if err := json.Unmarshal(row[i], dst); err != nil {
			return Kline{}, fmt.Errorf("field %d: %w", i, err)
		}
	}
	k := Kline{OpenTime: time.UnixMilli(openMs).UTC(), CloseTime: time.UnixMilli(closeMs).UTC()}
	for _, f := range []struct {
		s   string
		dst *money.Dec
	}{{o, &k.Open}, {h, &k.High}, {l, &k.Low}, {cl, &k.Close}, {v, &k.Volume}} {
		d, err := money.Parse(f.s)
		if err != nil {
			return Kline{}, err
		}
		*f.dst = d
	}
	return k, nil
}

// TickerPrices returns the last price of every symbol (one request, weight 4).
func (c *Client) TickerPrices(ctx context.Context) (map[string]money.Dec, error) {
	var raw []struct {
		Symbol string    `json:"symbol"`
		Price  money.Dec `json:"price"`
	}
	if err := c.do(ctx, request{method: http.MethodGet, path: "/api/v3/ticker/price"}, &raw); err != nil {
		return nil, err
	}
	out := make(map[string]money.Dec, len(raw))
	for _, r := range raw {
		out[r.Symbol] = r.Price
	}
	return out, nil
}

// TickerPrice returns the last price of one symbol.
func (c *Client) TickerPrice(ctx context.Context, symbol string) (money.Dec, error) {
	var raw struct {
		Price money.Dec `json:"price"`
	}
	err := c.do(ctx, request{method: http.MethodGet, path: "/api/v3/ticker/price", params: url.Values{"symbol": {symbol}}}, &raw)
	return raw.Price, err
}

// SymbolInfo is the part of exchangeInfo the strategies need.
type SymbolInfo struct {
	Symbol                 string
	Status                 string // TRADING, BREAK, ...
	BaseAsset              string
	QuoteAsset             string
	IsSpotTradingAllowed   bool
	IsMarginTradingAllowed bool
	StepSize               money.Dec // LOT_SIZE
	MinQty                 money.Dec
	MinNotional            money.Dec // NOTIONAL or MIN_NOTIONAL
}

type rawSymbol struct {
	Symbol                 string `json:"symbol"`
	Status                 string `json:"status"`
	BaseAsset              string `json:"baseAsset"`
	QuoteAsset             string `json:"quoteAsset"`
	IsSpotTradingAllowed   bool   `json:"isSpotTradingAllowed"`
	IsMarginTradingAllowed bool   `json:"isMarginTradingAllowed"`
	Filters                []struct {
		FilterType  string `json:"filterType"`
		StepSize    string `json:"stepSize"`
		MinQty      string `json:"minQty"`
		MinNotional string `json:"minNotional"`
	} `json:"filters"`
}

// ExchangeInfo returns symbol rules. One unknown symbol fails the whole call with -1121, so ask
// for symbols you know exist, or call it per symbol.
func (c *Client) ExchangeInfo(ctx context.Context, symbols ...string) ([]SymbolInfo, error) {
	p := url.Values{"showPermissionSets": {"false"}}
	switch len(symbols) {
	case 0:
	case 1:
		p.Set("symbol", symbols[0])
	default:
		b, _ := json.Marshal(symbols)
		p.Set("symbols", string(b))
	}
	var raw struct {
		Symbols []rawSymbol `json:"symbols"`
	}
	if err := c.do(ctx, request{method: http.MethodGet, path: "/api/v3/exchangeInfo", params: p}, &raw); err != nil {
		return nil, err
	}
	out := make([]SymbolInfo, 0, len(raw.Symbols))
	for _, s := range raw.Symbols {
		info := SymbolInfo{
			Symbol: s.Symbol, Status: s.Status, BaseAsset: s.BaseAsset, QuoteAsset: s.QuoteAsset,
			IsSpotTradingAllowed: s.IsSpotTradingAllowed, IsMarginTradingAllowed: s.IsMarginTradingAllowed,
		}
		for _, f := range s.Filters {
			switch f.FilterType {
			case "LOT_SIZE":
				info.StepSize, _ = money.Parse(f.StepSize)
				info.MinQty, _ = money.Parse(f.MinQty)
			case "NOTIONAL", "MIN_NOTIONAL":
				info.MinNotional, _ = money.Parse(f.MinNotional)
			}
		}
		out = append(out, info)
	}
	return out, nil
}
