package marketdata

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"bigwaves/internal/exchange/binance"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/events"
	"bigwaves/internal/platform/money"
	"bigwaves/internal/platform/scheduler"
)

func (s *Service) setPrice(ctx context.Context, q *dbq.Queries, id int64, price money.Dec) error {
	p := money.RoundPrice(price)
	now := dbtype.Now()
	return q.SetPairPrice(ctx, dbq.SetPairPriceParams{CurrentPrice: &p, PriceUpdatedAt: &now, ID: id})
}

// UpdatePrices stores the last price of every live pair, from one ticker call.
func (s *Service) UpdatePrices(ctx context.Context) (string, error) {
	pairs, err := dbq.New(s.DB.Read).ListLivePairNames(ctx)
	if err != nil {
		return "", err
	}
	if len(pairs) == 0 {
		return "", scheduler.Skipf("no pairs")
	}
	prices, err := s.Binance.TickerPrices(ctx)
	if err != nil {
		return "", err
	}
	q := dbq.New(s.DB.Write)
	updated, missing := 0, []string{}
	for _, p := range pairs {
		price, ok := prices[p.Name]
		if !ok {
			missing = append(missing, p.Name)
			continue
		}
		if err := s.setPrice(ctx, q, p.ID, price); err != nil {
			return "", err
		}
		updated++
	}
	msg := fmt.Sprintf("%d pairs updated", updated)
	if len(missing) > 0 {
		msg += fmt.Sprintf("; no ticker for %v", missing)
	}
	return msg, nil
}

// UpdateInterestRates stores each margin pair's yearly borrow rate of its base asset.
func (s *Service) UpdateInterestRates(ctx context.Context) (string, error) {
	pairs, err := s.List(ctx, ListFilter{})
	if err != nil {
		return "", err
	}
	q := dbq.New(s.DB.Write)
	since := time.Now().Add(-24 * time.Hour)
	updated := 0
	var errs []error
	for _, p := range pairs {
		if p.IsSpot {
			continue
		}
		daily, ok, err := s.Binance.DailyInterestRate(ctx, p.BaseAsset(), since)
		if err != nil {
			if errors.Is(err, binance.ErrBanned) || ctx.Err() != nil {
				return "", err
			}
			errs = append(errs, fmt.Errorf("%s: %w", p.Name, err))
			continue
		}
		if !ok {
			continue
		}
		yearly := daily.Mul(money.FromInt(365 * 100)).Round(4)
		if err := q.SetPairInterestRate(ctx, dbq.SetPairInterestRateParams{InterestRate: &yearly, ID: p.ID}); err != nil {
			return "", err
		}
		updated++
		sleep(ctx, 100*time.Millisecond)
	}
	msg := fmt.Sprintf("%d pairs updated", updated)
	if len(errs) > 0 {
		return msg, fmt.Errorf("%s, %d failed: %w", msg, len(errs), errors.Join(errs...))
	}
	return msg, nil
}

const filterBatch = 100

// UpdateFilters refreshes LOT_SIZE step and min notional for every live pair, 100 symbols per
// call. An unknown symbol fails its whole batch (-1121), so that batch is retried one by one.
func (s *Service) UpdateFilters(ctx context.Context) (string, error) {
	pairs, err := dbq.New(s.DB.Read).ListLivePairNames(ctx)
	if err != nil {
		return "", err
	}
	byName := map[string]int64{}
	names := make([]string, 0, len(pairs))
	for _, p := range pairs {
		byName[p.Name] = p.ID
		names = append(names, p.Name)
	}
	infos := map[string]binance.SymbolInfo{}
	var unknown []string
	for start := 0; start < len(names); start += filterBatch {
		batch := names[start:min(start+filterBatch, len(names))]
		got, err := s.Binance.ExchangeInfo(ctx, batch...)
		if binance.HasCode(err, binance.CodeBadSymbol) {
			for _, n := range batch {
				one, err := s.Binance.ExchangeInfo(ctx, n)
				if err != nil || len(one) == 0 {
					unknown = append(unknown, n)
					continue
				}
				infos[n] = one[0]
			}
			continue
		}
		if err != nil {
			return "", err
		}
		for _, i := range got {
			infos[i.Symbol] = i
		}
	}
	q := dbq.New(s.DB.Write)
	for name, info := range infos {
		if err := q.SetPairFilters(ctx, dbq.SetPairFiltersParams{StepSize: &info.StepSize, MinNotional: &info.MinNotional, ID: byName[name]}); err != nil {
			return "", err
		}
	}
	msg := fmt.Sprintf("%d pairs updated", len(infos))
	if len(unknown) > 0 {
		slog.WarnContext(ctx, "pairs missing from exchangeInfo", "pairs", unknown)
		msg += fmt.Sprintf("; unknown on Binance: %v", unknown)
	}
	return msg, nil
}

// Jobs returns the scheduled jobs of this package.
func (s *Service) Jobs(ev *events.Broker) []scheduler.Job {
	return []scheduler.Job{
		{
			Name: "market.update_prices", Description: "Cập nhật giá mọi pair (1 lệnh ticker)",
			Cron: "5 */10 * * * *", Timeout: 2 * time.Minute,
			Run: func(ctx context.Context) (string, error) {
				msg, err := s.UpdatePrices(ctx)
				if err == nil {
					ev.Publish("prices.updated", nil)
				}
				return msg, err
			},
		},
		{
			Name: "market.update_interest", Description: "Cập nhật lãi vay margin theo năm của từng coin",
			Cron: "0 3 * * * *", Timeout: 10 * time.Minute, Run: s.UpdateInterestRates,
		},
		{
			Name: "market.update_filters", Description: "Cập nhật step size và min notional từ exchangeInfo",
			Cron: "CRON_TZ=Asia/Ho_Chi_Minh 0 7 7 * * *", Timeout: 10 * time.Minute, Run: s.UpdateFilters,
		},
	}
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
