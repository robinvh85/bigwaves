package ema1d

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"bigwaves/internal/exchange/binance"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/money"
)

// feeRate is Binance's taker fee, used only when an order's fills cannot be read.
var feeRate = money.MustParse("0.001")

// filled is a filled order with what the strategy needs from it.
type filled struct {
	Order    binance.Order
	AvgPrice money.Dec
	NetQty   money.Dec // base asset received after base-asset commissions (buys)
	FeeUSDT  money.Dec
}

// fillsOf returns the fills of an order. A status query carries none, so they are read from the
// trade list (more exact than the Rails estimate at the taker rate).
func (s *Service) fillsOf(ctx context.Context, o binance.Order) []binance.Fill {
	if len(o.Fills) > 0 || o.ExecutedQty.IsZero() {
		return o.Fills
	}
	trades, err := s.Binance.OrderTrades(ctx, binance.Cross, o.Symbol, o.OrderID)
	if err != nil || len(trades) == 0 {
		return nil
	}
	out := make([]binance.Fill, len(trades))
	for i, t := range trades {
		out[i] = binance.Fill{Price: t.Price, Qty: t.Qty, Commission: t.Commission, CommissionAsset: t.CommissionAsset}
	}
	return out
}

func (s *Service) describeFill(ctx context.Context, o binance.Order, baseAsset string) (filled, error) {
	if o.ExecutedQty.IsZero() {
		return filled{}, fmt.Errorf("order %s: executedQty is 0", o.ClientOrderID)
	}
	f := filled{Order: o, AvgPrice: o.AvgPrice(), NetQty: o.ExecutedQty}
	fills := s.fillsOf(ctx, o)
	if len(fills) == 0 {
		f.FeeUSDT = o.CummulativeQuoteQty.Mul(feeRate)
		return f, nil
	}
	fee := money.Zero
	for _, fl := range fills {
		switch strings.ToUpper(fl.CommissionAsset) {
		case "USDT":
			fee = fee.Add(fl.Commission)
		case strings.ToUpper(baseAsset):
			fee = fee.Add(fl.Commission.Mul(fl.Price))
			f.NetQty = f.NetQty.Sub(fl.Commission)
		case "BNB":
			bnb, err := s.Binance.TickerPrice(ctx, "BNBUSDT")
			if err != nil {
				fee = fee.Add(fl.Qty.Mul(fl.Price).Mul(feeRate))
			} else {
				fee = fee.Add(fl.Commission.Mul(bnb))
			}
		default:
			fee = fee.Add(fl.Qty.Mul(fl.Price).Mul(feeRate))
		}
	}
	f.FeeUSDT = fee.Round(8)
	return f, nil
}

// lookupOrder finds an order placed by an earlier attempt. A found order that is not FILLED is an
// error: a MARKET order should be filled at once, so something unusual happened.
func (s *Service) lookupOrder(ctx context.Context, symbol, clientID string) (*binance.Order, error) {
	o, found, err := s.Binance.OrderByClientID(ctx, binance.Cross, symbol, clientID)
	if err != nil || !found {
		return nil, err
	}
	if !o.Filled() {
		return nil, fmt.Errorf("order %s exists on Binance but status=%s (expected FILLED)", clientID, o.Status)
	}
	return &o, nil
}

func (s *Service) marketOrder(ctx context.Context, symbol string, side binance.Side, qty money.Dec, clientID string) (binance.Order, error) {
	o, err := s.Binance.PlaceMarketOrder(ctx, binance.MarketOrder{
		Account: binance.Cross, Symbol: symbol, Side: side, Quantity: qty, ClientOrderID: clientID,
	})
	if err != nil {
		return o, err
	}
	if !o.Filled() {
		return o, fmt.Errorf("%s %s %s: order status %s", side, qty, symbol, o.Status)
	}
	return o, nil
}

// recordOrder keeps the order in exchange_orders (audit and cutover rollback). Best effort: the
// position itself is the source of truth for the strategy.
func (s *Service) recordOrder(ctx context.Context, positionID int64, f filled) {
	raw, _ := json.Marshal(f.Order)
	_ = dbq.New(s.DB.Write).UpsertExchangeOrder(ctx, dbq.UpsertExchangeOrderParams{
		ClientOrderID: f.Order.ClientOrderID, Strategy: StrategyKey, RefID: &positionID, Symbol: f.Order.Symbol,
		Account: string(binance.Cross), Side: string(f.Order.Side), ExchangeOrderID: &f.Order.OrderID,
		Status: f.Order.Status, ExecutedQty: f.Order.ExecutedQty, QuoteQty: f.Order.CummulativeQuoteQty,
		AvgPrice: f.AvgPrice, NetQty: f.NetQty, FeeUsdt: f.FeeUSDT, Raw: dbtype.JSON(raw),
		ExecutedAt: dbtype.NewTime(f.Order.ExecutedAt()),
	})
}
