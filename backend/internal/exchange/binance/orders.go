package binance

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"bigwaves/internal/platform/money"
)

type Side string

const (
	Buy  Side = "BUY"
	Sell Side = "SELL"
)

// Account selects where an order goes.
type Account string

const (
	Spot     Account = "SPOT"
	Cross    Account = "CROSS"
	Isolated Account = "ISOLATED"
)

// MarketOrder is a MARKET order. Set exactly one of Quantity (base asset) or QuoteQty (USDT).
type MarketOrder struct {
	Account       Account
	Symbol        string
	Side          Side
	Quantity      money.Dec
	QuoteQty      money.Dec
	ClientOrderID string
}

type Fill struct {
	Price           money.Dec `json:"price"`
	Qty             money.Dec `json:"qty"`
	Commission      money.Dec `json:"commission"`
	CommissionAsset string    `json:"commissionAsset"`
}

// Order is the order as Binance reports it. Fills are only present in the response to placing
// the order (newOrderRespType=FULL); a later status query has none.
type Order struct {
	Symbol              string    `json:"symbol"`
	OrderID             int64     `json:"orderId"`
	ClientOrderID       string    `json:"clientOrderId"`
	TransactTime        int64     `json:"transactTime"`
	Time                int64     `json:"time"`
	Status              string    `json:"status"` // NEW, FILLED, PARTIALLY_FILLED, CANCELED, REJECTED, EXPIRED
	Type                string    `json:"type"`
	Side                Side      `json:"side"`
	OrigQty             money.Dec `json:"origQty"`
	ExecutedQty         money.Dec `json:"executedQty"`
	CummulativeQuoteQty money.Dec `json:"cummulativeQuoteQty"`
	Fills               []Fill    `json:"fills"`
}

func (o Order) Filled() bool { return o.Status == "FILLED" }

// AvgPrice is the volume-weighted fill price: quote spent / quantity executed.
func (o Order) AvgPrice() money.Dec {
	if o.ExecutedQty.IsZero() {
		return money.Zero
	}
	return o.CummulativeQuoteQty.Div(o.ExecutedQty)
}

// ExecutedAt is the transaction time of the order.
func (o Order) ExecutedAt() time.Time {
	ms := o.TransactTime
	if ms == 0 {
		ms = o.Time
	}
	return time.UnixMilli(ms).UTC()
}

func orderPath(a Account) string {
	if a == Spot {
		return "/api/v3/order"
	}
	return "/sapi/v1/margin/order"
}

// PlaceMarketOrder sends a MARKET order. It is a write call: blocked unless trading is enabled.
func (c *Client) PlaceMarketOrder(ctx context.Context, o MarketOrder) (Order, error) {
	p := url.Values{
		"symbol":           {o.Symbol},
		"side":             {string(o.Side)},
		"type":             {"MARKET"},
		"newOrderRespType": {"FULL"},
	}
	if !o.Quantity.IsZero() {
		p.Set("quantity", o.Quantity.String())
	}
	if !o.QuoteQty.IsZero() {
		p.Set("quoteOrderQty", o.QuoteQty.String())
	}
	if o.ClientOrderID != "" {
		p.Set("newClientOrderId", o.ClientOrderID)
	}
	if o.Account != Spot {
		p.Set("isIsolated", strconv.FormatBool(o.Account == Isolated))
	}
	var out Order
	err := c.do(ctx, request{method: http.MethodPost, path: orderPath(o.Account), params: p, security: signed, write: true}, &out)
	return out, err
}

// OrderByClientID looks an order up by clientOrderId. found is false when Binance has no such
// order (-2013), which means a previous attempt never reached the exchange.
func (c *Client) OrderByClientID(ctx context.Context, account Account, symbol, clientOrderID string) (order Order, found bool, err error) {
	p := url.Values{"symbol": {symbol}, "origClientOrderId": {clientOrderID}}
	if account != Spot {
		p.Set("isIsolated", strconv.FormatBool(account == Isolated))
	}
	err = c.do(ctx, request{method: http.MethodGet, path: orderPath(account), params: p, security: signed}, &order)
	if HasCode(err, CodeNoSuchOrder) {
		return Order{}, false, nil
	}
	if err != nil {
		return Order{}, false, err
	}
	return order, true, nil
}

// Trade is one execution of an order, with its commission (myTrades / margin myTrades).
type Trade struct {
	OrderID         int64     `json:"orderId"`
	Price           money.Dec `json:"price"`
	Qty             money.Dec `json:"qty"`
	QuoteQty        money.Dec `json:"quoteQty"`
	Commission      money.Dec `json:"commission"`
	CommissionAsset string    `json:"commissionAsset"`
	Time            int64     `json:"time"`
}

// OrderTrades returns the executions of one order, used to recover fills and fees for an order
// found by OrderByClientID (whose response carries no fills).
func (c *Client) OrderTrades(ctx context.Context, account Account, symbol string, orderID int64) ([]Trade, error) {
	p := url.Values{"symbol": {symbol}, "orderId": {strconv.FormatInt(orderID, 10)}}
	path := "/api/v3/myTrades"
	if account != Spot {
		path = "/sapi/v1/margin/myTrades"
		p.Set("isIsolated", strconv.FormatBool(account == Isolated))
	}
	var out []Trade
	err := c.do(ctx, request{method: http.MethodGet, path: path, params: p, security: signed}, &out)
	return out, err
}
