package ema1d

import (
	"context"
	"fmt"
	"strings"

	"bigwaves/internal/exchange/binance"
	"bigwaves/internal/platform/db/dbq"
)

// PairCheck is the result of checking one pair on Binance.
type PairCheck struct {
	Coin           string `json:"coin"`
	ShortSupported bool   `json:"shortSupported"`
	CrossMargin    bool   `json:"crossMargin"`
	Note           string `json:"note"`
}

// CheckPairs records what each EMA1D pair can do on Binance:
//   - the symbol trades on cross margin (allPairs: isMarginTrade, buy and sell allowed)
//   - the base coin can be borrowed (allAssets: isBorrowable) → short_supported
//   - LOT_SIZE step and min notional (exchangeInfo), cached on the pair
//   - how much could be borrowed right now (maxBorrowable), as a note only
//
// A pair that loses short support gets a Discord alert. ids nil checks every non-disabled pair.
func (s *Service) CheckPairs(ctx context.Context, ids []int64) ([]PairCheck, error) {
	pairs, err := s.listPairs(ctx, dbq.New(s.DB.Read))
	if err != nil {
		return nil, err
	}
	marginPairs, err := s.Binance.CrossMarginPairs(ctx)
	if err != nil {
		return nil, err
	}
	assets, err := s.Binance.CrossMarginAssets(ctx)
	if err != nil {
		return nil, err
	}
	mp := map[string]binance.MarginPair{}
	for _, p := range marginPairs {
		mp[p.Symbol] = p
	}
	as := map[string]binance.MarginAssetInfo{}
	for _, a := range assets {
		as[a.AssetName] = a
	}

	var out []PairCheck
	for _, pv := range pairs {
		if ids == nil && pv.Status == PairDisabled {
			continue
		}
		if ids != nil && !contains(ids, pv.ID) {
			continue
		}
		out = append(out, s.checkPair(ctx, pv, mp, as))
	}
	s.changed()
	return out, nil
}

func contains(ids []int64, id int64) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func (s *Service) checkPair(ctx context.Context, pv pairView, mp map[string]binance.MarginPair, as map[string]binance.MarginAssetInfo) PairCheck {
	m, inMargin := mp[pv.PairName]
	crossOK := inMargin && m.IsMarginTrade && m.IsBuyAllowed && m.IsSellAllowed
	a, known := as[pv.CoinName]
	borrowable := known && a.IsBorrowable

	notes := []string{"cross margin: ok"}
	if !crossOK {
		notes[0] = "cross margin: NOT tradable"
	}
	if borrowable {
		notes = append(notes, "borrowable: yes")
	} else {
		notes = append(notes, "borrowable: NO")
	}
	notes = append(notes, s.lotNotes(ctx, pv)...)
	if borrowable {
		if amt, err := s.Binance.MaxBorrowable(ctx, pv.CoinName); err != nil {
			notes = append(notes, "max borrowable: "+err.Error())
		} else {
			notes = append(notes, fmt.Sprintf("max borrowable now: %s %s", amt, pv.CoinName))
		}
	}
	note := strings.Join(notes, "; ")
	supported := crossOK && borrowable
	q := dbq.New(s.DB.Write)
	_ = q.SetEma1dPairCheck(ctx, dbq.SetEma1dPairCheckParams{ShortSupported: supported, CheckedAt: tptr(s.now()), CheckNote: note, ID: pv.ID})

	var problems []string
	if pv.ShortSupported && !supported && pv.AllowShort {
		problems = append(problems, "can no longer be shorted ("+note+")")
	}
	if !crossOK {
		problems = append(problems, "is not tradable on cross margin")
	}
	if len(problems) > 0 {
		s.notify(ctx, fmt.Sprintf("EMA1D: %s %s", pv.PairName, strings.Join(problems, " and ")))
	}
	return PairCheck{Coin: pv.CoinName, ShortSupported: supported, CrossMargin: crossOK, Note: note}
}

func (s *Service) lotNotes(ctx context.Context, pv pairView) []string {
	infos, err := s.Binance.ExchangeInfo(ctx, pv.PairName)
	if err != nil {
		return []string{"exchange info: " + err.Error()}
	}
	if len(infos) == 0 {
		return []string{"exchange info: missing"}
	}
	info := infos[0]
	_ = dbq.New(s.DB.Write).SetPairFilters(ctx, dbq.SetPairFiltersParams{StepSize: &info.StepSize, MinNotional: &info.MinNotional, ID: pv.PairID})
	return []string{"step " + info.StepSize.String(), "min notional " + info.MinNotional.String() + " USDT"}
}
