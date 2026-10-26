package ema1d

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"bigwaves/internal/marketdata"
	"bigwaves/internal/platform/db/dbq"
)

// The basket chosen in October 2026: core and sensitive coins, long-only coins, and coins whose
// LONG exits on XO (the others exit on DIV4H).
var (
	CoreCoins      = []string{"JASMY", "WLD", "RAY", "GRT", "SUN", "BONK", "THETA", "STX", "SEI", "RENDER", "ETH", "FLOKI", "PEPE", "ZEC", "ARB"}
	SensitiveCoins = []string{"AR", "BTC", "FET", "RUNE", "CFX"}
	LongOnlyCoins  = []string{"BTC", "ZEC"}
	LongExitXO     = []string{"BTC"}
)

type SeedReport struct {
	CreatedPairs  []string `json:"createdPairs"`
	CreatedBasket []string `json:"createdBasket"`
	KeptBasket    []string `json:"keptBasket"`
	Warnings      []string `json:"warnings"`
}

// Seed sets up the basket idempotently: creates the missing pairs (Binance cross margin) and
// ema1d_pairs rows. Existing rows are left as they are, so edits made on the screen survive.
// addedOn is the first candle date whose entries are taken.
func (s *Service) Seed(ctx context.Context, addedOn time.Time) (SeedReport, error) {
	rep := SeedReport{CreatedPairs: []string{}, CreatedBasket: []string{}, KeptBasket: []string{}, Warnings: []string{}}
	added := dateOnly(addedOn)
	for _, coin := range append(slices.Clone(CoreCoins), SensitiveCoins...) {
		name := coin + "USDT"
		pairRow, err := dbq.New(s.DB.Read).GetPairByName(ctx, name)
		if isNoRows(err) {
			p, err := s.Pairs.Create(ctx, marketdata.CreateInput{Name: name})
			if err != nil {
				rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: %v", name, err))
				continue
			}
			rep.CreatedPairs = append(rep.CreatedPairs, name)
			pairRow = p.Pair
		} else if err != nil {
			return rep, err
		}
		if pairRow.IsSpot || pairRow.IsIsolated || pairRow.DeletedAt != nil || pairRow.Status != "ACTIVE" {
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: not an active cross-margin pair", name))
		}
		if _, err := dbq.New(s.DB.Read).GetEma1dPairByPairID(ctx, pairRow.ID); err == nil {
			rep.KeptBasket = append(rep.KeptBasket, coin)
			continue
		} else if !isNoRows(err) {
			return rep, err
		}
		longOnly := slices.Contains(LongOnlyCoins, coin)
		group := GroupCore
		if slices.Contains(SensitiveCoins, coin) {
			group = GroupSensitive
		}
		rule := "DIV4H"
		if slices.Contains(LongExitXO, coin) {
			rule = "XO"
		}
		note := "short_supported seeded, not checked on Binance yet"
		if longOnly {
			note = "long only"
		}
		if _, err := dbq.New(s.DB.Write).CreateEma1dPair(ctx, dbq.CreateEma1dPairParams{
			PairID: pairRow.ID, PairGroup: group, AllowLong: true, AllowShort: !longOnly, ShortSupported: !longOnly,
			Status: PairActive, AddedOn: added, LongExitRule: &rule, CheckNote: note,
		}); err != nil {
			return rep, err
		}
		rep.CreatedBasket = append(rep.CreatedBasket, coin)
	}
	if len(rep.CreatedBasket) == 0 && len(rep.KeptBasket) == 0 {
		return rep, errors.New("no basket pair could be set up")
	}
	s.changed()
	return rep, nil
}
