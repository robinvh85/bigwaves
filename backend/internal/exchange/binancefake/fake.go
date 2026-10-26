// Package binancefake is an in-memory Binance for tests: market data, spot and cross-margin
// balances, MARKET orders, borrow/repay, and error injection. It speaks the same HTTP API as the
// real exchange, so tests exercise the real client.
package binancefake

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"bigwaves/internal/platform/money"
)

type Symbol struct {
	Symbol      string
	Base        string
	Quote       string
	StepSize    string
	MinNotional string
	Margin      bool // cross-margin tradable
	Borrowable  bool // base asset can be borrowed
}

type Balance struct {
	Free, Locked, Borrowed, Interest money.Dec
}

type order struct {
	Symbol        string
	OrderID       int64
	ClientOrderID string
	Side          string
	Account       string
	Qty, Quote    money.Dec
	Price         money.Dec
	Commission    money.Dec
	CommAsset     string
	Time          int64
}

type loan struct {
	Type      string
	Asset     string
	Amount    money.Dec
	Timestamp int64
	TxID      int64
}

// Failure makes the next N calls to Path fail.
type Failure struct {
	Path   string // e.g. "/sapi/v1/margin/order"
	Method string // optional
	Times  int
	HTTP   int
	Code   int
	Msg    string
	// AfterApply applies the call and then fails: the request reached the exchange but the
	// response was lost.
	AfterApply bool
}

type Exchange struct {
	t   testing.TB
	srv *httptest.Server

	mu          sync.Mutex
	Symbols     map[string]Symbol
	Prices      map[string]money.Dec
	Spot        map[string]*Balance
	Cross       map[string]*Balance
	MarginLevel money.Dec
	DailyRates  map[string]money.Dec
	// FeeRate is the commission charged on the received asset (default 0.001).
	FeeRate  money.Dec
	orders   map[string]*order
	loans    []loan
	nextID   int64
	failures []*Failure
	Calls    []string
	Now      func() time.Time
}

func New(t testing.TB) *Exchange {
	e := &Exchange{
		t:           t,
		Symbols:     map[string]Symbol{},
		Prices:      map[string]money.Dec{},
		Spot:        map[string]*Balance{},
		Cross:       map[string]*Balance{},
		MarginLevel: money.FromInt(999),
		DailyRates:  map[string]money.Dec{},
		FeeRate:     money.MustParse("0.001"),
		orders:      map[string]*order{},
		nextID:      1000,
		Now:         time.Now,
	}
	e.srv = httptest.NewServer(http.HandlerFunc(e.serve))
	t.Cleanup(e.srv.Close)
	return e
}

func (e *Exchange) URL() string { return e.srv.URL }

// AddSymbol registers a symbol with a price.
func (e *Exchange) AddSymbol(s Symbol, price string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if s.Quote == "" {
		s.Quote = "USDT"
	}
	if s.Base == "" {
		s.Base = strings.TrimSuffix(s.Symbol, s.Quote)
	}
	if s.StepSize == "" {
		s.StepSize = "0.01"
	}
	if s.MinNotional == "" {
		s.MinNotional = "5"
	}
	e.Symbols[s.Symbol] = s
	e.Prices[s.Symbol] = money.MustParse(price)
}

func (e *Exchange) SetPrice(symbol, price string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Prices[symbol] = money.MustParse(price)
}

func bal(m map[string]*Balance, asset string) *Balance {
	b, ok := m[asset]
	if !ok {
		b = &Balance{}
		m[asset] = b
	}
	return b
}

// FundCross sets the free cross-margin balance of asset.
func (e *Exchange) FundCross(asset, free string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	bal(e.Cross, asset).Free = money.MustParse(free)
}

func (e *Exchange) FundSpot(asset, free string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	bal(e.Spot, asset).Free = money.MustParse(free)
}

// CrossBalance returns a copy of a cross-margin balance.
func (e *Exchange) CrossBalance(asset string) Balance {
	e.mu.Lock()
	defer e.mu.Unlock()
	return *bal(e.Cross, asset)
}

// AccrueInterest adds interest to a borrowed asset.
func (e *Exchange) AccrueInterest(asset, amount string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	bal(e.Cross, asset).Interest = bal(e.Cross, asset).Interest.Add(money.MustParse(amount))
}

func (e *Exchange) Fail(f Failure) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if f.Times == 0 {
		f.Times = 1
	}
	if f.HTTP == 0 {
		f.HTTP = http.StatusBadRequest
	}
	e.failures = append(e.failures, &f)
}

// OrderCount returns how many orders were placed.
func (e *Exchange) OrderCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.orders)
}

// CallCount counts calls whose "METHOD path" starts with prefix.
func (e *Exchange) CallCount(prefix string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, c := range e.Calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

func (e *Exchange) takeFailure(r *http.Request) *Failure {
	for i, f := range e.failures {
		if f.Path == r.URL.Path && (f.Method == "" || f.Method == r.Method) {
			f.Times--
			if f.Times <= 0 {
				e.failures = append(e.failures[:i], e.failures[i+1:]...)
			}
			return f
		}
	}
	return nil
}

func writeErr(w http.ResponseWriter, status, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "msg": msg})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-MBX-USED-WEIGHT-1M", "10")
	_ = json.NewEncoder(w).Encode(v)
}

func (e *Exchange) serve(w http.ResponseWriter, r *http.Request) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Calls = append(e.Calls, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
	q := r.URL.Query()

	if f := e.takeFailure(r); f != nil {
		if !f.AfterApply {
			writeErr(w, f.HTTP, f.Code, f.Msg)
			return
		}
		e.route(httptest.NewRecorder(), r, q)
		writeErr(w, f.HTTP, f.Code, f.Msg)
		return
	}
	e.route(w, r, q)
}

func (e *Exchange) route(w http.ResponseWriter, r *http.Request, q map[string][]string) {
	get := func(k string) string {
		if v := q[k]; len(v) > 0 {
			return v[0]
		}
		return ""
	}
	switch r.Method + " " + r.URL.Path {
	case "GET /api/v3/time":
		writeJSON(w, map[string]int64{"serverTime": e.Now().UnixMilli()})
	case "GET /api/v3/ticker/price":
		e.tickers(w, get("symbol"))
	case "GET /api/v3/exchangeInfo":
		e.exchangeInfo(w, get("symbol"), get("symbols"))
	case "GET /api/v3/klines":
		writeJSON(w, []any{})
	case "GET /api/v3/account":
		e.spotAccount(w)
	case "GET /sapi/v1/margin/account":
		e.crossAccount(w)
	case "POST /api/v3/order", "POST /sapi/v1/margin/order":
		e.placeOrder(w, r.URL.Path, get)
	case "GET /api/v3/order", "GET /sapi/v1/margin/order":
		e.getOrder(w, get("origClientOrderId"))
	case "GET /api/v3/myTrades", "GET /sapi/v1/margin/myTrades":
		e.myTrades(w, get("orderId"))
	case "POST /sapi/v1/margin/borrow-repay":
		e.borrowRepay(w, get)
	case "GET /sapi/v1/margin/borrow-repay":
		e.loanRecords(w, get("type"), get("asset"), get("startTime"))
	case "POST /sapi/v1/asset/transfer":
		e.transfer(w, get)
	case "GET /sapi/v1/margin/allPairs":
		e.allPairs(w)
	case "GET /sapi/v1/margin/allAssets":
		e.allAssets(w)
	case "GET /sapi/v1/margin/maxBorrowable":
		writeJSON(w, map[string]string{"amount": "1000000", "borrowLimit": "1000000"})
	case "GET /sapi/v1/margin/interestRateHistory":
		e.interest(w, get("asset"))
	default:
		writeErr(w, http.StatusNotFound, -1, "fake: no route "+r.Method+" "+r.URL.Path)
	}
}

func (e *Exchange) tickers(w http.ResponseWriter, symbol string) {
	if symbol != "" {
		p, ok := e.Prices[symbol]
		if !ok {
			writeErr(w, 400, -1121, "Invalid symbol.")
			return
		}
		writeJSON(w, map[string]string{"symbol": symbol, "price": p.String()})
		return
	}
	out := []map[string]string{}
	for s, p := range e.Prices {
		out = append(out, map[string]string{"symbol": s, "price": p.String()})
	}
	writeJSON(w, out)
}

func (e *Exchange) exchangeInfo(w http.ResponseWriter, symbol, symbols string) {
	var names []string
	if symbol != "" {
		names = []string{symbol}
	} else if symbols != "" {
		_ = json.Unmarshal([]byte(symbols), &names)
	} else {
		for n := range e.Symbols {
			names = append(names, n)
		}
	}
	out := []any{}
	for _, n := range names {
		s, ok := e.Symbols[n]
		if !ok {
			writeErr(w, 400, -1121, "Invalid symbol.")
			return
		}
		out = append(out, map[string]any{
			"symbol": s.Symbol, "status": "TRADING", "baseAsset": s.Base, "quoteAsset": s.Quote,
			"isSpotTradingAllowed": true, "isMarginTradingAllowed": s.Margin,
			"filters": []map[string]string{
				{"filterType": "LOT_SIZE", "stepSize": s.StepSize, "minQty": s.StepSize},
				{"filterType": "NOTIONAL", "minNotional": s.MinNotional},
			},
		})
	}
	writeJSON(w, map[string]any{"symbols": out})
}

func (e *Exchange) spotAccount(w http.ResponseWriter) {
	out := []map[string]string{}
	for a, b := range e.Spot {
		out = append(out, map[string]string{"asset": a, "free": b.Free.String(), "locked": b.Locked.String()})
	}
	writeJSON(w, map[string]any{"balances": out})
}

func (e *Exchange) crossAccount(w http.ResponseWriter) {
	out := []map[string]string{}
	for a, b := range e.Cross {
		net := b.Free.Add(b.Locked).Sub(b.Borrowed).Sub(b.Interest)
		out = append(out, map[string]string{
			"asset": a, "free": b.Free.String(), "locked": b.Locked.String(), "borrowed": b.Borrowed.String(),
			"interest": b.Interest.String(), "netAsset": net.String(),
		})
	}
	writeJSON(w, map[string]any{
		"marginLevel": e.MarginLevel.String(), "tradeEnabled": true, "borrowEnabled": true,
		"totalAssetOfBtc": "0", "totalLiabilityOfBtc": "0", "totalNetAssetOfBtc": "0", "userAssets": out,
	})
}

func (e *Exchange) placeOrder(w http.ResponseWriter, path string, get func(string) string) {
	sym, ok := e.Symbols[get("symbol")]
	if !ok {
		writeErr(w, 400, -1121, "Invalid symbol.")
		return
	}
	cid := get("newClientOrderId")
	if _, dup := e.orders[cid]; dup && cid != "" {
		writeErr(w, 400, -2010, "Duplicate order sent.")
		return
	}
	price := e.Prices[sym.Symbol]
	var qty, quote money.Dec
	if s := get("quantity"); s != "" {
		qty = money.MustParse(s)
		quote = qty.Mul(price)
	} else {
		quote = money.MustParse(get("quoteOrderQty"))
		qty = money.FloorToStep(quote.Div(price), money.MustParse(sym.StepSize))
		quote = qty.Mul(price)
	}
	wallet := e.Cross
	account := "CROSS"
	if path == "/api/v3/order" {
		wallet, account = e.Spot, "SPOT"
	}
	side := get("side")
	var commission money.Dec
	var commAsset string
	switch side {
	case "BUY":
		if bal(wallet, sym.Quote).Free.LessThan(quote) {
			writeErr(w, 400, -2010, "Account has insufficient balance for requested action.")
			return
		}
		commission, commAsset = qty.Mul(e.FeeRate), sym.Base
		bal(wallet, sym.Quote).Free = bal(wallet, sym.Quote).Free.Sub(quote)
		bal(wallet, sym.Base).Free = bal(wallet, sym.Base).Free.Add(qty).Sub(commission)
	case "SELL":
		if bal(wallet, sym.Base).Free.LessThan(qty) {
			writeErr(w, 400, -2010, "Account has insufficient balance for requested action.")
			return
		}
		commission, commAsset = quote.Mul(e.FeeRate), sym.Quote
		bal(wallet, sym.Base).Free = bal(wallet, sym.Base).Free.Sub(qty)
		bal(wallet, sym.Quote).Free = bal(wallet, sym.Quote).Free.Add(quote).Sub(commission)
	}
	e.nextID++
	o := &order{
		Symbol: sym.Symbol, OrderID: e.nextID, ClientOrderID: cid, Side: side, Account: account,
		Qty: qty, Quote: quote, Price: price, Commission: commission, CommAsset: commAsset, Time: e.Now().UnixMilli(),
	}
	if cid == "" {
		o.ClientOrderID = fmt.Sprintf("auto_%d", o.OrderID)
	}
	e.orders[o.ClientOrderID] = o
	writeJSON(w, e.orderJSON(o, true))
}

func (e *Exchange) orderJSON(o *order, withFills bool) map[string]any {
	m := map[string]any{
		"symbol": o.Symbol, "orderId": o.OrderID, "clientOrderId": o.ClientOrderID, "transactTime": o.Time, "time": o.Time,
		"status": "FILLED", "type": "MARKET", "side": o.Side, "origQty": o.Qty.String(),
		"executedQty": o.Qty.String(), "cummulativeQuoteQty": o.Quote.String(),
	}
	if withFills {
		m["fills"] = []map[string]string{{
			"price": o.Price.String(), "qty": o.Qty.String(), "commission": o.Commission.String(), "commissionAsset": o.CommAsset,
		}}
	}
	return m
}

func (e *Exchange) getOrder(w http.ResponseWriter, cid string) {
	o, ok := e.orders[cid]
	if !ok {
		writeErr(w, 400, -2013, "Order does not exist.")
		return
	}
	writeJSON(w, e.orderJSON(o, false))
}

func (e *Exchange) myTrades(w http.ResponseWriter, orderID string) {
	id, _ := strconv.ParseInt(orderID, 10, 64)
	for _, o := range e.orders {
		if o.OrderID == id {
			writeJSON(w, []map[string]any{{
				"orderId": o.OrderID, "price": o.Price.String(), "qty": o.Qty.String(), "quoteQty": o.Quote.String(),
				"commission": o.Commission.String(), "commissionAsset": o.CommAsset, "time": o.Time,
			}})
			return
		}
	}
	writeJSON(w, []any{})
}

func (e *Exchange) borrowRepay(w http.ResponseWriter, get func(string) string) {
	asset, typ := get("asset"), get("type")
	amount := money.MustParse(get("amount"))
	b := bal(e.Cross, asset)
	switch typ {
	case "BORROW":
		borrowable := asset == "USDT"
		for _, s := range e.Symbols {
			if s.Base == asset && s.Borrowable {
				borrowable = true
			}
		}
		if !borrowable {
			writeErr(w, 400, -3003, "This asset is not borrowable.")
			return
		}
		b.Free = b.Free.Add(amount)
		b.Borrowed = b.Borrowed.Add(amount)
	case "REPAY":
		if b.Free.LessThan(amount) {
			writeErr(w, 400, -3041, "Balance is not enough.")
			return
		}
		debt := b.Borrowed.Add(b.Interest)
		pay := money.Min(amount, debt)
		b.Free = b.Free.Sub(pay)
		fromInterest := money.Min(pay, b.Interest)
		b.Interest = b.Interest.Sub(fromInterest)
		b.Borrowed = b.Borrowed.Sub(pay.Sub(fromInterest))
	default:
		writeErr(w, 400, -1102, "bad type")
		return
	}
	e.nextID++
	e.loans = append(e.loans, loan{Type: typ, Asset: asset, Amount: amount, Timestamp: e.Now().UnixMilli(), TxID: e.nextID})
	writeJSON(w, map[string]int64{"tranId": e.nextID})
}

func (e *Exchange) loanRecords(w http.ResponseWriter, typ, asset, start string) {
	since, _ := strconv.ParseInt(start, 10, 64)
	rows := []map[string]any{}
	for _, l := range e.loans {
		if l.Type == typ && l.Asset == asset && l.Timestamp >= since {
			rows = append(rows, map[string]any{
				"type": l.Type, "asset": l.Asset, "amount": l.Amount.String(), "principal": l.Amount.String(),
				"interest": "0", "status": "CONFIRMED", "timestamp": l.Timestamp, "txId": l.TxID,
			})
		}
	}
	writeJSON(w, map[string]any{"rows": rows, "total": len(rows)})
}

func (e *Exchange) transfer(w http.ResponseWriter, get func(string) string) {
	asset, amount := get("asset"), money.MustParse(get("amount"))
	from, to := e.Spot, e.Cross
	if get("type") == "MARGIN_MAIN" {
		from, to = e.Cross, e.Spot
	}
	if bal(from, asset).Free.LessThan(amount) {
		writeErr(w, 400, -5002, "insufficient balance")
		return
	}
	bal(from, asset).Free = bal(from, asset).Free.Sub(amount)
	bal(to, asset).Free = bal(to, asset).Free.Add(amount)
	e.nextID++
	writeJSON(w, map[string]int64{"tranId": e.nextID})
}

func (e *Exchange) allPairs(w http.ResponseWriter) {
	out := []map[string]any{}
	for _, s := range e.Symbols {
		out = append(out, map[string]any{
			"symbol": s.Symbol, "base": s.Base, "quote": s.Quote,
			"isMarginTrade": s.Margin, "isBuyAllowed": s.Margin, "isSellAllowed": s.Margin,
		})
	}
	writeJSON(w, out)
}

func (e *Exchange) allAssets(w http.ResponseWriter) {
	out := []map[string]any{{"assetName": "USDT", "isBorrowable": true, "isMortgageable": true}}
	for _, s := range e.Symbols {
		out = append(out, map[string]any{"assetName": s.Base, "isBorrowable": s.Borrowable, "isMortgageable": true})
	}
	writeJSON(w, out)
}

func (e *Exchange) interest(w http.ResponseWriter, asset string) {
	rate, ok := e.DailyRates[asset]
	if !ok {
		writeJSON(w, []any{})
		return
	}
	writeJSON(w, []map[string]any{{"asset": asset, "dailyInterestRate": rate.String(), "timestamp": e.Now().UnixMilli()}})
}
