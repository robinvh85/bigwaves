package httpapi

import (
	"context"

	"bigwaves/internal/exchange/binance"
	"bigwaves/internal/platform/httpx"
)

func (s *Server) GetBalances(ctx context.Context, req GetBalancesRequestObject) (GetBalancesResponseObject, error) {
	b, err := s.Account.Balances(ctx, deref(req.Params.Refresh))
	if err != nil {
		return nil, err
	}
	return GetBalances200JSONResponse{
		SpotUsdtFree:   b.SpotUSDTFree,
		CrossUsdtFree:  b.CrossUSDTFree,
		CrossUsdtDebt:  b.CrossUSDTDebt,
		CrossDebtUsdt:  b.CrossDebtUSDT,
		MarginLevel:    b.MarginLevel,
		SpotTotalUsdt:  b.SpotTotalUSDT,
		CrossTotalUsdt: b.CrossTotalUSDT,
		NetTotalUsdt:   b.NetTotalUSDT,
		FetchedAt:      b.FetchedAt,
	}, nil
}

func (s *Server) BorrowUsdt(ctx context.Context, req BorrowUsdtRequestObject) (BorrowUsdtResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("missing body", nil)
	}
	id, err := s.Account.BorrowUSDT(ctx, req.Body.Amount)
	if err != nil {
		return nil, err
	}
	return BorrowUsdt200JSONResponse{TranId: id}, nil
}

func (s *Server) RepayUsdt(ctx context.Context, req RepayUsdtRequestObject) (RepayUsdtResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("missing body", nil)
	}
	id, err := s.Account.RepayUSDT(ctx, req.Body.Amount)
	if err != nil {
		return nil, err
	}
	return RepayUsdt200JSONResponse{TranId: id}, nil
}

func (s *Server) TransferUsdt(ctx context.Context, req TransferUsdtRequestObject) (TransferUsdtResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("missing body", nil)
	}
	dir := binance.SpotToCross
	if req.Body.Direction == CROSSTOSPOT {
		dir = binance.CrossToSpot
	}
	id, err := s.Account.TransferUSDT(ctx, dir, req.Body.Amount)
	if err != nil {
		return nil, err
	}
	return TransferUsdt200JSONResponse{TranId: id}, nil
}
