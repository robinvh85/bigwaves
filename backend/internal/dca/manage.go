package dca

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/httpx"
	"bigwaves/internal/platform/money"
)

// TradeRow is one rung for the API.
type TradeRow struct {
	ID               int64      `json:"id"`
	SettingID        int64      `json:"settingId"`
	Status           string     `json:"status"`
	BuyPrice         money.Dec  `json:"buyPrice"`
	SellPrice        money.Dec  `json:"sellPrice"`
	Amount           money.Dec  `json:"amount"`
	HeldAmount       money.Dec  `json:"heldAmount"`
	OpenPrice        *money.Dec `json:"openPrice,omitempty"`
	ClosePrice       *money.Dec `json:"closePrice,omitempty"`
	CostUsdt         money.Dec  `json:"costUsdt"`
	FeeUsdt          money.Dec  `json:"feeUsdt"`
	RealizedProfit   money.Dec  `json:"realizedProfit"`
	UnrealizedProfit money.Dec  `json:"unrealizedProfit"`
	LastError        string     `json:"lastError"`
	OpenAt           *time.Time `json:"openAt,omitempty"`
	CloseAt          *time.Time `json:"closeAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
}

// SettingRow is a ladder with its totals.
type SettingRow struct {
	ID               int64      `json:"id"`
	PairID           int64      `json:"pairId"`
	PairName         string     `json:"pairName"`
	Status           string     `json:"status"`
	Note             string     `json:"note"`
	CurrentPrice     *money.Dec `json:"currentPrice,omitempty"`
	PriceUpdatedAt   *time.Time `json:"priceUpdatedAt,omitempty"`
	StepSize         *money.Dec `json:"stepSize,omitempty"`
	MinNotional      *money.Dec `json:"minNotional,omitempty"`
	Account          string     `json:"account"`
	TradeCount       int        `json:"tradeCount"`
	WaitingCount     int        `json:"waitingCount"`
	HoldingCount     int        `json:"holdingCount"`
	HeldAmount       money.Dec  `json:"heldAmount"`
	CapitalUsdt      money.Dec  `json:"capitalUsdt"`
	RealizedProfit   money.Dec  `json:"realizedProfit"`
	UnrealizedProfit money.Dec  `json:"unrealizedProfit"`
	NextBuyPrice     *money.Dec `json:"nextBuyPrice,omitempty"`
	NextSellPrice    *money.Dec `json:"nextSellPrice,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	Trades           []TradeRow `json:"trades,omitempty"`
}

func tp(t *time.Time) *time.Time { return t }

func tradeRow(t dbq.DcaTrade) TradeRow {
	r := TradeRow{
		ID: t.ID, SettingID: t.DcaSettingID, Status: t.Status, BuyPrice: t.BuyPrice, SellPrice: t.SellPrice, Amount: t.Amount,
		HeldAmount: t.HeldAmount, OpenPrice: t.OpenPrice, ClosePrice: t.ClosePrice, CostUsdt: t.BuyQuote, FeeUsdt: t.FeeUsdt,
		RealizedProfit: t.RealizedProfit, UnrealizedProfit: t.UnrealizedProfit, LastError: t.LastError, CreatedAt: t.CreatedAt.Time,
	}
	if t.OpenAt != nil {
		r.OpenAt = tp(&t.OpenAt.Time)
	}
	if t.CloseAt != nil {
		r.CloseAt = tp(&t.CloseAt.Time)
	}
	return r
}

func (s *Service) row(ctx context.Context, v settingView, withTrades bool) (SettingRow, error) {
	r := SettingRow{
		ID: v.ID, PairID: v.PairID, PairName: v.PairName, Status: v.Status, Note: v.Note, CurrentPrice: v.CurrentPrice,
		PriceUpdatedAt: dbtype.TimePtr(v.PriceUpdatedAt), StepSize: v.StepSize, MinNotional: v.MinNotional, Account: string(v.account()), CreatedAt: v.CreatedAt.Time,
	}
	trades, err := dbq.New(s.DB.Read).ListDcaTrades(ctx, v.ID)
	if err != nil {
		return r, err
	}
	r.TradeCount = len(trades)
	for _, t := range trades {
		r.RealizedProfit = r.RealizedProfit.Add(t.RealizedProfit)
		switch t.Status {
		case TradeOpening:
			r.WaitingCount++
			if r.NextBuyPrice == nil || t.BuyPrice.GreaterThan(*r.NextBuyPrice) {
				bp := t.BuyPrice
				r.NextBuyPrice = &bp
			}
		case TradeOpen:
			r.HoldingCount++
			r.HeldAmount = r.HeldAmount.Add(t.HeldAmount)
			r.CapitalUsdt = r.CapitalUsdt.Add(t.BuyQuote)
			r.UnrealizedProfit = r.UnrealizedProfit.Add(t.UnrealizedProfit)
			if t.SellPrice.IsPositive() && (r.NextSellPrice == nil || t.SellPrice.LessThan(*r.NextSellPrice)) {
				sp := t.SellPrice
				r.NextSellPrice = &sp
			}
		}
		if withTrades {
			r.Trades = append(r.Trades, tradeRow(t))
		}
	}
	if withTrades && r.Trades == nil {
		r.Trades = []TradeRow{}
	}
	return r, nil
}

// List returns the ladders with their totals; status "" lists all.
func (s *Service) List(ctx context.Context, status string) ([]SettingRow, error) {
	var st any
	if status != "" {
		st = status
	}
	rows, err := dbq.New(s.DB.Read).ListDcaSettings(ctx, st)
	if err != nil {
		return nil, err
	}
	out := make([]SettingRow, 0, len(rows))
	for _, r := range rows {
		row, err := s.row(ctx, settingView{r.DcaSetting, r.PairName, r.CurrentPrice, r.PriceUpdatedAt, r.StepSize, r.MinNotional, r.IsSpot, r.IsIsolated}, false)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, id int64) (SettingRow, error) {
	v, err := s.getSetting(ctx, dbq.New(s.DB.Read), id)
	if err != nil {
		return SettingRow{}, err
	}
	return s.row(ctx, v, true)
}

func (s *Service) CreateSetting(ctx context.Context, pairID int64, note string) (SettingRow, error) {
	p, err := dbq.New(s.DB.Read).GetPair(ctx, pairID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && p.Pair.DeletedAt != nil) {
		return SettingRow{}, httpx.Validation("pair does not exist", nil)
	}
	if err != nil {
		return SettingRow{}, err
	}
	row, err := dbq.New(s.DB.Write).CreateDcaSetting(ctx, dbq.CreateDcaSettingParams{PairID: pairID, Note: note})
	if err != nil {
		return SettingRow{}, err
	}
	s.changed()
	return s.Get(ctx, row.ID)
}

func (s *Service) UpdateSetting(ctx context.Context, id int64, status, note *string) (SettingRow, error) {
	v, err := s.getSetting(ctx, dbq.New(s.DB.Read), id)
	if err != nil {
		return SettingRow{}, err
	}
	st, n := v.Status, v.Note
	if status != nil {
		if *status != SettingActive && *status != SettingInactive && *status != SettingDone {
			return SettingRow{}, httpx.Validation("status must be ACTIVE, INACTIVE or DONE", nil)
		}
		st = *status
	}
	if note != nil {
		n = *note
	}
	if err := dbq.New(s.DB.Write).UpdateDcaSetting(ctx, dbq.UpdateDcaSettingParams{Status: st, Note: n, ID: id}); err != nil {
		return SettingRow{}, err
	}
	s.changed()
	return s.Get(ctx, id)
}

func (s *Service) DeleteSetting(ctx context.Context, id int64) error {
	row, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if row.HoldingCount > 0 {
		return httpx.Conflict("dca_has_holdings", "some rungs still hold coins: sell or settle them before deleting")
	}
	if err := dbq.New(s.DB.Write).DeleteDcaSetting(ctx, id); err != nil {
		return err
	}
	s.changed()
	return nil
}

// TradeInput creates or edits a rung. Nil fields are kept on edit.
type TradeInput struct {
	BuyPrice  *money.Dec
	SellPrice *money.Dec
	Amount    *money.Dec
}

func validateRung(v settingView, buy, sell, amount money.Dec) error {
	details := map[string]any{}
	if !buy.IsPositive() {
		details["buyPrice"] = "must be greater than 0"
	}
	if !amount.IsPositive() {
		details["amount"] = "must be greater than 0"
	}
	if sell.IsNegative() || (sell.IsPositive() && sell.LessOrEqual(buy)) {
		details["sellPrice"] = "must be above the buy price (or 0 to hold)"
	}
	if st := step(v); st.IsPositive() && amount.IsPositive() && !money.FloorToStep(amount, st).IsPositive() {
		details["amount"] = fmt.Sprintf("smaller than the quantity step %s", st)
	}
	if v.MinNotional != nil && amount.Mul(buy).LessThan(*v.MinNotional) && amount.IsPositive() && buy.IsPositive() {
		details["amount"] = fmt.Sprintf("value %s USDT is below the Binance minimum of %s USDT", amount.Mul(buy).Round(2), v.MinNotional)
	}
	if len(details) > 0 {
		return httpx.Validation("invalid DCA rung", details)
	}
	return nil
}

func (s *Service) CreateTrade(ctx context.Context, settingID int64, in TradeInput) (TradeRow, error) {
	v, err := s.getSetting(ctx, dbq.New(s.DB.Read), settingID)
	if err != nil {
		return TradeRow{}, err
	}
	if in.BuyPrice == nil || in.Amount == nil {
		return TradeRow{}, httpx.Validation("buy price and amount are required", nil)
	}
	sell := money.Zero
	if in.SellPrice != nil {
		sell = *in.SellPrice
	}
	if err := validateRung(v, *in.BuyPrice, sell, *in.Amount); err != nil {
		return TradeRow{}, err
	}
	var t dbq.DcaTrade
	err = s.DB.Tx(ctx, func(tx *sql.Tx) error {
		q := dbq.New(tx)
		tmp := fmt.Sprintf("tmp_%d_%d", settingID, s.now().UnixNano())
		var err error
		t, err = q.CreateDcaTrade(ctx, dbq.CreateDcaTradeParams{
			DcaSettingID: settingID, BuyPrice: *in.BuyPrice, SellPrice: sell, Amount: *in.Amount,
			OpenOrderID: tmp + "_o", CloseOrderID: tmp + "_c",
		})
		if err != nil {
			return err
		}
		t.OpenOrderID, t.CloseOrderID = openOrderID(t.ID), closeOrderID(t.ID)
		return q.SetDcaTradeOrderIDs(ctx, dbq.SetDcaTradeOrderIDsParams{OpenOrderID: t.OpenOrderID, CloseOrderID: t.CloseOrderID, ID: t.ID})
	})
	if err != nil {
		return TradeRow{}, err
	}
	s.changed()
	return tradeRow(t), nil
}

func (s *Service) trade(ctx context.Context, id int64) (dbq.DcaTrade, settingView, error) {
	t, err := dbq.New(s.DB.Read).GetDcaTrade(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return t, settingView{}, httpx.NewError(404, "dca_trade_not_found", "DCA rung not found")
	}
	if err != nil {
		return t, settingView{}, err
	}
	v, err := s.getSetting(ctx, dbq.New(s.DB.Read), t.DcaSettingID)
	return t, v, err
}

// UpdateTrade edits a rung: everything while waiting to buy, only the sell price once held.
func (s *Service) UpdateTrade(ctx context.Context, id int64, in TradeInput) (TradeRow, error) {
	t, v, err := s.trade(ctx, id)
	if err != nil {
		return TradeRow{}, err
	}
	switch t.Status {
	case TradeOpening:
		if in.BuyPrice != nil {
			t.BuyPrice = *in.BuyPrice
		}
		if in.Amount != nil {
			t.Amount = *in.Amount
		}
		if in.SellPrice != nil {
			t.SellPrice = *in.SellPrice
		}
		if err := validateRung(v, t.BuyPrice, t.SellPrice, t.Amount); err != nil {
			return TradeRow{}, err
		}
	case TradeOpen:
		if in.BuyPrice != nil || in.Amount != nil {
			return TradeRow{}, httpx.Conflict("dca_trade_held", "rung already bought: only the sell price can change")
		}
		if in.SellPrice != nil {
			sp := *in.SellPrice
			if sp.IsNegative() || (sp.IsPositive() && t.OpenPrice != nil && sp.LessOrEqual(*t.OpenPrice)) {
				return TradeRow{}, httpx.Validation("sell price must be above the bought price (or 0 to hold)", nil)
			}
			t.SellPrice = sp
		}
	default:
		return TradeRow{}, httpx.Conflict("dca_trade_closed", "rung already sold, it cannot change")
	}
	if err := saveTrade(ctx, dbq.New(s.DB.Write), t); err != nil {
		return TradeRow{}, err
	}
	s.changed()
	return tradeRow(t), nil
}

func (s *Service) DeleteTrade(ctx context.Context, id int64) error {
	t, _, err := s.trade(ctx, id)
	if err != nil {
		return err
	}
	if t.Status != TradeOpening {
		return httpx.Conflict("dca_trade_not_waiting", "only rungs not yet bought can be deleted")
	}
	if err := dbq.New(s.DB.Write).DeleteDcaTrade(ctx, id); err != nil {
		return err
	}
	s.changed()
	return nil
}

// ForceBuy buys a waiting rung now, at market.
func (s *Service) ForceBuy(ctx context.Context, id int64) (TradeRow, error) {
	t, v, err := s.trade(ctx, id)
	if err != nil {
		return TradeRow{}, err
	}
	if t.Status != TradeOpening {
		return TradeRow{}, httpx.Conflict("dca_trade_not_waiting", "only rungs waiting to buy can be bought now")
	}
	cfg, _, err := s.Config(ctx)
	if err != nil {
		return TradeRow{}, err
	}
	if err := s.buy(ctx, v, &t, cfg); err != nil {
		_ = s.recordFailure(ctx, v, &t, err)
		return TradeRow{}, upstream(err)
	}
	s.changed()
	return tradeRow(t), nil
}

// ForceSell sells a held rung now, at market.
func (s *Service) ForceSell(ctx context.Context, id int64) (TradeRow, error) {
	t, v, err := s.trade(ctx, id)
	if err != nil {
		return TradeRow{}, err
	}
	if t.Status != TradeOpen {
		return TradeRow{}, httpx.Conflict("dca_trade_not_held", "only held rungs can be sold now")
	}
	if err := s.sell(ctx, v, &t); err != nil {
		_ = s.recordFailure(ctx, v, &t, err)
		return TradeRow{}, upstream(err)
	}
	s.changed()
	return tradeRow(t), nil
}

func upstream(err error) error {
	var he *httpx.Error
	if errors.As(err, &he) {
		return err
	}
	return &httpx.Error{Status: 502, Code: "exchange_error", Message: "order failed: " + err.Error(), Err: err}
}
