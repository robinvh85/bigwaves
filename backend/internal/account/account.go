// Package account reads Binance balances and moves USDT between spot and cross margin.
package account

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"bigwaves/internal/exchange/binance"
	"bigwaves/internal/platform/db"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/httpx"
	"bigwaves/internal/platform/money"
)

type Service struct {
	DB      *db.DB
	Binance *binance.Client
	TTL     time.Duration

	sf     singleflight.Group
	mu     sync.Mutex
	cached *Balances
	at     time.Time
}

// Balances is a USDT view of the account. Non-USDT holdings are valued with the pair prices
// stored in the database.
type Balances struct {
	SpotUSDTFree   money.Dec
	CrossUSDTFree  money.Dec
	CrossUSDTDebt  money.Dec // borrowed + interest
	CrossDebtUSDT  money.Dec // every borrowed asset valued in USDT
	MarginLevel    money.Dec
	SpotTotalUSDT  money.Dec
	CrossTotalUSDT money.Dec
	NetTotalUSDT   money.Dec // spot + cross - debt
	FetchedAt      time.Time
}

func (s *Service) ttl() time.Duration {
	if s.TTL == 0 {
		return 30 * time.Second
	}
	return s.TTL
}

// Balances returns cached balances (30s) unless refresh is set.
func (s *Service) Balances(ctx context.Context, refresh bool) (Balances, error) {
	s.mu.Lock()
	if !refresh && s.cached != nil && time.Since(s.at) < s.ttl() {
		b := *s.cached
		s.mu.Unlock()
		return b, nil
	}
	s.mu.Unlock()
	v, err, _ := s.sf.Do("balances", func() (any, error) { return s.fetch(ctx) })
	if err != nil {
		return Balances{}, upstream(err)
	}
	b := v.(Balances)
	s.mu.Lock()
	s.cached, s.at = &b, time.Now()
	s.mu.Unlock()
	return b, nil
}

func (s *Service) invalidate() {
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
}

func (s *Service) fetch(ctx context.Context) (Balances, error) {
	spot, err := s.Binance.SpotBalances(ctx)
	if err != nil {
		return Balances{}, err
	}
	cross, err := s.Binance.CrossMarginAccount(ctx)
	if err != nil {
		return Balances{}, err
	}
	prices, err := s.prices(ctx)
	if err != nil {
		return Balances{}, err
	}
	valueOf := func(asset string, amount money.Dec) money.Dec {
		if asset == "USDT" {
			return amount
		}
		if p, ok := prices[asset+"USDT"]; ok {
			return amount.Mul(p)
		}
		return money.Zero
	}

	b := Balances{MarginLevel: cross.MarginLevel, FetchedAt: time.Now().UTC()}
	for _, a := range spot {
		if a.Asset == "USDT" {
			b.SpotUSDTFree = a.Free
		}
		b.SpotTotalUSDT = b.SpotTotalUSDT.Add(valueOf(a.Asset, a.Free.Add(a.Locked)))
	}
	for _, a := range cross.UserAssets {
		if a.Asset == "USDT" {
			b.CrossUSDTFree = a.Free
			b.CrossUSDTDebt = a.Debt()
		}
		b.CrossTotalUSDT = b.CrossTotalUSDT.Add(valueOf(a.Asset, a.Free.Add(a.Locked)))
		if a.Borrowed.IsPositive() {
			b.CrossDebtUSDT = b.CrossDebtUSDT.Add(valueOf(a.Asset, a.Debt()))
		}
	}
	b.NetTotalUSDT = b.SpotTotalUSDT.Add(b.CrossTotalUSDT).Sub(b.CrossDebtUSDT)
	for _, d := range []*money.Dec{&b.SpotTotalUSDT, &b.CrossTotalUSDT, &b.CrossDebtUSDT, &b.NetTotalUSDT} {
		*d = d.Round(2)
	}
	return b, nil
}

func (s *Service) prices(ctx context.Context) (map[string]money.Dec, error) {
	rows, err := dbq.New(s.DB.Read).PricesByNames(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]money.Dec, len(rows))
	for _, r := range rows {
		if r.CurrentPrice != nil {
			out[r.Name] = *r.CurrentPrice
		}
	}
	return out, nil
}

func positive(amount money.Dec) error {
	if !amount.IsPositive() {
		return httpx.Validation("số tiền phải lớn hơn 0", map[string]any{"amount": amount.String()})
	}
	return nil
}

// BorrowUSDT borrows USDT on cross margin.
func (s *Service) BorrowUSDT(ctx context.Context, amount money.Dec) (int64, error) {
	if err := positive(amount); err != nil {
		return 0, err
	}
	defer s.invalidate()
	id, err := s.Binance.BorrowRepay(ctx, binance.Borrow, "USDT", amount, false, "")
	return id, upstream(err)
}

// RepayUSDT repays USDT debt on cross margin.
func (s *Service) RepayUSDT(ctx context.Context, amount money.Dec) (int64, error) {
	if err := positive(amount); err != nil {
		return 0, err
	}
	defer s.invalidate()
	id, err := s.Binance.BorrowRepay(ctx, binance.Repay, "USDT", amount, false, "")
	return id, upstream(err)
}

// TransferUSDT moves USDT between spot and cross margin.
func (s *Service) TransferUSDT(ctx context.Context, dir binance.TransferDirection, amount money.Dec) (int64, error) {
	if err := positive(amount); err != nil {
		return 0, err
	}
	defer s.invalidate()
	id, err := s.Binance.Transfer(ctx, dir, "USDT", amount)
	return id, upstream(err)
}

func upstream(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, binance.ErrNotConfigured) {
		return &httpx.Error{Status: http.StatusFailedDependency, Code: "exchange_not_configured", Message: "chưa cấu hình API key Binance", Err: err}
	}
	if errors.Is(err, binance.ErrTradingDisabled) {
		return &httpx.Error{Status: http.StatusConflict, Code: "trading_disabled", Message: "TRADING_ENABLED đang tắt: không gửi lệnh ghi lên Binance", Err: err}
	}
	return &httpx.Error{Status: http.StatusBadGateway, Code: "exchange_error", Message: "lỗi khi gọi Binance: " + err.Error(), Err: err}
}
