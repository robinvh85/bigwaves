// Package marketdata owns pairs and coins, and the jobs that keep their prices, borrow rates and
// order filters up to date.
package marketdata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"bigwaves/internal/exchange/binance"
	"bigwaves/internal/platform/db"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/httpx"
)

type Service struct {
	DB      *db.DB
	Binance *binance.Client
}

// Pair is a pair with its coin, as the API returns it.
type Pair struct {
	dbq.Pair
	CoinName        string
	CoinDescription string
}

func (p Pair) BaseAsset() string { return strings.TrimSuffix(p.Name, p.QuoteAsset) }

var symbolRe = regexp.MustCompile(`^[A-Z0-9]{2,20}$`)

var errPairNotFound = httpx.NewError(http.StatusNotFound, "pair_not_found", "pair not found")

type ListFilter struct {
	Status         string // "", ACTIVE, DISABLED
	IncludeDeleted bool
}

func (s *Service) List(ctx context.Context, f ListFilter) ([]Pair, error) {
	params := dbq.ListPairsParams{}
	if f.Status != "" {
		params.Status = f.Status
	}
	if f.IncludeDeleted {
		params.IncludeDeleted = 1
	}
	rows, err := dbq.New(s.DB.Read).ListPairs(ctx, params)
	if err != nil {
		return nil, err
	}
	out := make([]Pair, len(rows))
	for i, r := range rows {
		out[i] = Pair{Pair: r.Pair, CoinName: r.CoinName, CoinDescription: r.CoinDescription}
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, id int64) (Pair, error) {
	r, err := dbq.New(s.DB.Read).GetPair(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Pair{}, errPairNotFound
	}
	if err != nil {
		return Pair{}, err
	}
	return Pair{Pair: r.Pair, CoinName: r.CoinName, CoinDescription: r.CoinDescription}, nil
}

type CreateInput struct {
	Name       string
	IsSpot     bool
	IsIsolated bool
}

// Create adds a pair by its Binance symbol. The symbol is checked against exchangeInfo, the coin
// is created from the base asset, and the order filters are stored right away. A soft-deleted
// pair with the same name is restored instead.
func (s *Service) Create(ctx context.Context, in CreateInput) (Pair, error) {
	name := strings.ToUpper(strings.TrimSpace(in.Name))
	if !symbolRe.MatchString(name) {
		return Pair{}, httpx.Validation("invalid pair name", map[string]any{"name": "uppercase letters and digits only, e.g. SUNUSDT"})
	}
	infos, err := s.Binance.ExchangeInfo(ctx, name)
	if binance.HasCode(err, binance.CodeBadSymbol) || (err == nil && len(infos) == 0) {
		return Pair{}, httpx.Validation("Binance has no such symbol", map[string]any{"name": name})
	}
	if err != nil {
		return Pair{}, upstream(err)
	}
	info := infos[0]
	if in.IsSpot && in.IsIsolated {
		return Pair{}, httpx.Validation("a spot pair cannot be isolated margin", nil)
	}
	if !in.IsSpot && !info.IsMarginTradingAllowed {
		return Pair{}, httpx.Validation("this symbol is not margin-tradable on Binance; choose spot", map[string]any{"name": name})
	}

	var id int64
	err = s.DB.Tx(ctx, func(tx *sql.Tx) error {
		q := dbq.New(tx)
		coin, err := q.CreateCoin(ctx, info.BaseAsset)
		if err != nil {
			return err
		}
		existing, err := q.GetPairByName(ctx, name)
		switch {
		case err == nil && existing.DeletedAt == nil:
			return httpx.Conflict("pair_exists", fmt.Sprintf("pair %s already exists", name))
		case err == nil:
			p, err := q.RestorePair(ctx, dbq.RestorePairParams{
				Status: "ACTIVE", StepSize: &info.StepSize, MinNotional: &info.MinNotional,
				IsSpot: in.IsSpot, IsIsolated: in.IsIsolated, ID: existing.ID,
			})
			id = p.ID
			return err
		case errors.Is(err, sql.ErrNoRows):
			p, err := q.CreatePair(ctx, dbq.CreatePairParams{
				CoinID: coin.ID, Name: name, QuoteAsset: info.QuoteAsset, Status: "ACTIVE",
				StepSize: &info.StepSize, MinNotional: &info.MinNotional, IsSpot: in.IsSpot, IsIsolated: in.IsIsolated,
			})
			id = p.ID
			return err
		default:
			return err
		}
	})
	if err != nil {
		return Pair{}, err
	}
	// Best effort: a first price so the pair is usable before the next price job.
	if price, err := s.Binance.TickerPrice(ctx, name); err == nil {
		_ = s.setPrice(ctx, dbq.New(s.DB.Write), id, price)
	}
	return s.Get(ctx, id)
}

type UpdateInput struct {
	Status     *string
	IsSpot     *bool
	IsIsolated *bool
}

func (s *Service) Update(ctx context.Context, id int64, in UpdateInput) (Pair, error) {
	cur, err := s.Get(ctx, id)
	if err != nil {
		return Pair{}, err
	}
	if cur.DeletedAt != nil {
		return Pair{}, errPairNotFound
	}
	status, isSpot, isIsolated := cur.Status, cur.IsSpot, cur.IsIsolated
	if in.Status != nil {
		if *in.Status != "ACTIVE" && *in.Status != "DISABLED" {
			return Pair{}, httpx.Validation("status must be ACTIVE or DISABLED", nil)
		}
		status = *in.Status
	}
	if in.IsSpot != nil {
		isSpot = *in.IsSpot
	}
	if in.IsIsolated != nil {
		isIsolated = *in.IsIsolated
	}
	if isSpot && isIsolated {
		return Pair{}, httpx.Validation("a spot pair cannot be isolated margin", nil)
	}
	if _, err := dbq.New(s.DB.Write).UpdatePair(ctx, dbq.UpdatePairParams{Status: status, IsSpot: isSpot, IsIsolated: isIsolated, ID: id}); err != nil {
		return Pair{}, err
	}
	return s.Get(ctx, id)
}

// Delete soft-deletes a pair. Strategies that still reference it keep their history.
func (s *Service) Delete(ctx context.Context, id int64) error {
	n, err := dbq.New(s.DB.Write).SoftDeletePair(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return errPairNotFound
	}
	return nil
}

func (s *Service) UpdateCoinDescription(ctx context.Context, coinID int64, description string) (dbq.Coin, error) {
	if len(description) > 100_000 {
		return dbq.Coin{}, httpx.Validation("description is too long", nil)
	}
	c, err := dbq.New(s.DB.Write).UpdateCoinDescription(ctx, dbq.UpdateCoinDescriptionParams{Description: description, ID: coinID})
	if errors.Is(err, sql.ErrNoRows) {
		return dbq.Coin{}, httpx.NewError(http.StatusNotFound, "coin_not_found", "coin not found")
	}
	return c, err
}

// upstream wraps an exchange failure as a 502 for the API.
func upstream(err error) error {
	if errors.Is(err, binance.ErrNotConfigured) {
		return &httpx.Error{Status: http.StatusFailedDependency, Code: "exchange_not_configured", Message: "Binance API key is not configured", Err: err}
	}
	return &httpx.Error{Status: http.StatusBadGateway, Code: "exchange_error", Message: "Binance call failed: " + err.Error(), Err: err}
}
