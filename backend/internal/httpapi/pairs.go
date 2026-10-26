package httpapi

import (
	"context"

	"bigwaves/internal/marketdata"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/httpx"
)

func toPair(p marketdata.Pair) Pair {
	out := Pair{
		Id:           p.ID,
		Name:         p.Name,
		QuoteAsset:   p.QuoteAsset,
		Status:       PairStatus(p.Status),
		Coin:         Coin{Id: p.CoinID, Name: p.CoinName, Description: p.CoinDescription},
		CurrentPrice: p.CurrentPrice,
		InterestRate: p.InterestRate,
		StepSize:     p.StepSize,
		MinNotional:  p.MinNotional,
		IsSpot:       p.IsSpot,
		IsIsolated:   p.IsIsolated,
		CreatedAt:    p.CreatedAt.Time,
	}
	if p.PriceUpdatedAt != nil {
		out.PriceUpdatedAt = &p.PriceUpdatedAt.Time
	}
	if p.DeletedAt != nil {
		out.DeletedAt = &p.DeletedAt.Time
	}
	return out
}

func (s *Server) ListPairs(ctx context.Context, req ListPairsRequestObject) (ListPairsResponseObject, error) {
	f := marketdata.ListFilter{IncludeDeleted: deref(req.Params.IncludeDeleted)}
	if req.Params.Status != nil {
		f.Status = string(*req.Params.Status)
	}
	pairs, err := s.Pairs.List(ctx, f)
	if err != nil {
		return nil, err
	}
	items := make([]Pair, len(pairs))
	for i, p := range pairs {
		items[i] = toPair(p)
	}
	return ListPairs200JSONResponse{Items: items}, nil
}

func (s *Server) GetPair(ctx context.Context, req GetPairRequestObject) (GetPairResponseObject, error) {
	p, err := s.Pairs.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return GetPair200JSONResponse(toPair(p)), nil
}

func (s *Server) CreatePair(ctx context.Context, req CreatePairRequestObject) (CreatePairResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("thiếu body", nil)
	}
	p, err := s.Pairs.Create(ctx, marketdata.CreateInput{
		Name: req.Body.Name, IsSpot: deref(req.Body.IsSpot), IsIsolated: deref(req.Body.IsIsolated),
	})
	if err != nil {
		return nil, err
	}
	return CreatePair201JSONResponse(toPair(p)), nil
}

func (s *Server) UpdatePair(ctx context.Context, req UpdatePairRequestObject) (UpdatePairResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("thiếu body", nil)
	}
	in := marketdata.UpdateInput{IsSpot: req.Body.IsSpot, IsIsolated: req.Body.IsIsolated}
	if req.Body.Status != nil {
		st := string(*req.Body.Status)
		in.Status = &st
	}
	p, err := s.Pairs.Update(ctx, req.Id, in)
	if err != nil {
		return nil, err
	}
	return UpdatePair200JSONResponse(toPair(p)), nil
}

func (s *Server) DeletePair(ctx context.Context, req DeletePairRequestObject) (DeletePairResponseObject, error) {
	if err := s.Pairs.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return DeletePair204Response{}, nil
}

func (s *Server) UpdateCoin(ctx context.Context, req UpdateCoinRequestObject) (UpdateCoinResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("thiếu body", nil)
	}
	c, err := s.Pairs.UpdateCoinDescription(ctx, req.Id, req.Body.Description)
	if err != nil {
		return nil, err
	}
	return UpdateCoin200JSONResponse(toCoin(c)), nil
}

func toCoin(c dbq.Coin) Coin { return Coin{Id: c.ID, Name: c.Name, Description: c.Description} }

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
