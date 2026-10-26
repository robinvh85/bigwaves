package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"bigwaves/internal/auth"
	"bigwaves/internal/ema1d"
	"bigwaves/internal/platform/configstore"
	"bigwaves/internal/platform/httpx"
)

// convert copies a domain value into its generated API type. The domain types carry the same
// JSON names as api/openapi.yaml (checked by TestDomainTypesMatchTheAPI).
func convert[T any](src any) (T, error) {
	var dst T
	b, err := json.Marshal(src)
	if err != nil {
		return dst, err
	}
	if err := json.Unmarshal(b, &dst); err != nil {
		return dst, fmt.Errorf("convert %T: %w", src, err)
	}
	return dst, nil
}

func (s *Server) GetEma1dStatus(ctx context.Context, _ GetEma1dStatusRequestObject) (GetEma1dStatusResponseObject, error) {
	st, err := s.EMA1D.Status(ctx)
	if err != nil {
		return nil, err
	}
	return convert[GetEma1dStatus200JSONResponse](st)
}

func (s *Server) ListEma1dPairs(ctx context.Context, _ ListEma1dPairsRequestObject) (ListEma1dPairsResponseObject, error) {
	rows, err := s.EMA1D.PairRows(ctx)
	if err != nil {
		return nil, err
	}
	return convert[ListEma1dPairs200JSONResponse](map[string]any{"items": rows})
}

func (s *Server) UpdateEma1dPair(ctx context.Context, req UpdateEma1dPairRequestObject) (UpdateEma1dPairResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("missing body", nil)
	}
	b := req.Body
	in := ema1d.PairUpdate{
		AllowLong: b.AllowLong, AllowShort: b.AllowShort, RemovedOn: b.RemovedOn, LongExitRule: b.LongExitRule,
		SignalFrom: b.SignalFrom,
	}
	if b.PairGroup != nil {
		v := string(*b.PairGroup)
		in.PairGroup = &v
	}
	if b.Status != nil {
		v := string(*b.Status)
		in.Status = &v
	}
	if b.AddedOn != nil {
		v := b.AddedOn.String()
		in.AddedOn = &v
	}
	row, err := s.EMA1D.UpdatePair(ctx, req.Id, in)
	if err != nil {
		return nil, err
	}
	return convert[UpdateEma1dPair200JSONResponse](row)
}

func (s *Server) CheckEma1dPairs(ctx context.Context, req CheckEma1dPairsRequestObject) (CheckEma1dPairsResponseObject, error) {
	var ids []int64
	if req.Body != nil && req.Body.Ids != nil {
		ids = *req.Body.Ids
	}
	res, err := s.EMA1D.CheckPairs(ctx, ids)
	if err != nil {
		return nil, exchangeErr(err)
	}
	return convert[CheckEma1dPairs200JSONResponse](map[string]any{"items": nonNilSlice(res)})
}

func (s *Server) ListEma1dTodos(ctx context.Context, req ListEma1dTodosRequestObject) (ListEma1dTodosResponseObject, error) {
	rows, err := s.EMA1D.Todos(ctx, deref(req.Params.History))
	if err != nil {
		return nil, err
	}
	return convert[ListEma1dTodos200JSONResponse](map[string]any{"items": rows})
}

func (s *Server) CreateEma1dTodo(ctx context.Context, req CreateEma1dTodoRequestObject) (CreateEma1dTodoResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("missing body", nil)
	}
	b := req.Body
	in := ema1d.ManualTodoRequest{
		Action: string(b.Action), Coin: b.Coin, RefPrice: deref(b.RefPrice), Force: deref(b.Force),
	}
	if b.Side != nil {
		in.Side = string(*b.Side)
	}
	if b.SignalDate != nil {
		in.SignalDate = b.SignalDate.String()
	}
	if b.Rule != nil {
		in.Rule = string(*b.Rule)
	}
	res, err := s.EMA1D.CreateManualTodo(ctx, in)
	if err != nil {
		return nil, err
	}
	return convert[CreateEma1dTodo200JSONResponse](res)
}

func (s *Server) RetryEma1dTodo(ctx context.Context, req RetryEma1dTodoRequestObject) (RetryEma1dTodoResponseObject, error) {
	row, err := s.EMA1D.RetryTodo(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return convert[RetryEma1dTodo200JSONResponse](row)
}

func (s *Server) CancelEma1dTodo(ctx context.Context, req CancelEma1dTodoRequestObject) (CancelEma1dTodoResponseObject, error) {
	row, err := s.EMA1D.CancelTodo(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return convert[CancelEma1dTodo200JSONResponse](row)
}

func (s *Server) ListEma1dTrades(ctx context.Context, req ListEma1dTradesRequestObject) (ListEma1dTradesResponseObject, error) {
	p := req.Params
	f := ema1d.TradesFilter{Coin: deref(p.Coin), Month: deref(p.Month)}
	if p.Status != nil && *p.Status == Closed {
		f.Closed = true
	}
	if p.Side != nil {
		f.Side = string(*p.Side)
	}
	rows, sum, err := s.EMA1D.Trades(ctx, f)
	if err != nil {
		return nil, err
	}
	return convert[ListEma1dTrades200JSONResponse](map[string]any{"items": rows, "summary": sum})
}

func (s *Server) GetEma1dEquity(ctx context.Context, req GetEma1dEquityRequestObject) (GetEma1dEquityResponseObject, error) {
	days := 180
	if req.Params.Days != nil {
		days = *req.Params.Days
	}
	pts, err := s.EMA1D.EquityCurve(ctx, days)
	if err != nil {
		return nil, err
	}
	return convert[GetEma1dEquity200JSONResponse](map[string]any{"items": pts})
}

func (s *Server) GetEma1dSignals(ctx context.Context, _ GetEma1dSignalsRequestObject) (GetEma1dSignalsResponseObject, error) {
	sig, err := s.EMA1D.Signals(ctx)
	if err != nil {
		return nil, httpx.Unprocessable("signals_failed", err.Error())
	}
	return convert[GetEma1dSignals200JSONResponse](sig)
}

func (s *Server) ListEma1dDailyChecks(ctx context.Context, req ListEma1dDailyChecksRequestObject) (ListEma1dDailyChecksResponseObject, error) {
	limit := int64(30)
	if req.Params.Limit != nil {
		limit = int64(*req.Params.Limit)
	}
	rows, err := s.EMA1D.DailyChecks(ctx, limit)
	if err != nil {
		return nil, err
	}
	return convert[ListEma1dDailyChecks200JSONResponse](map[string]any{"items": rows})
}

func (s *Server) configEnvelope(ctx context.Context) (Ema1dConfigEnvelope, error) {
	c, v, err := s.EMA1D.Config(ctx)
	if err != nil {
		return Ema1dConfigEnvelope{}, err
	}
	return convert[Ema1dConfigEnvelope](map[string]any{"config": c, "version": v, "suggestedSizes": ema1d.SuggestedSizes})
}

func (s *Server) GetEma1dConfig(ctx context.Context, _ GetEma1dConfigRequestObject) (GetEma1dConfigResponseObject, error) {
	env, err := s.configEnvelope(ctx)
	return GetEma1dConfig200JSONResponse(env), err
}

func (s *Server) UpdateEma1dConfig(ctx context.Context, req UpdateEma1dConfigRequestObject) (UpdateEma1dConfigResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("missing body", nil)
	}
	c, err := convert[ema1d.Config](req.Body.Config)
	if err != nil {
		return nil, httpx.Validation(err.Error(), nil)
	}
	u, _ := auth.UserFrom(ctx)
	if _, err := s.EMA1D.SaveConfig(ctx, req.Body.Version, c, u.Email); err != nil {
		if errors.Is(err, configstore.ErrVersionConflict) {
			return nil, httpx.Conflict("version_conflict", "the settings were just changed by someone else; reload and try again")
		}
		return nil, err
	}
	env, err := s.configEnvelope(ctx)
	return UpdateEma1dConfig200JSONResponse(env), err
}

func (s *Server) GetEma1dConfigHistory(ctx context.Context, _ GetEma1dConfigHistoryRequestObject) (GetEma1dConfigHistoryResponseObject, error) {
	rows, err := s.Configs.History(ctx, ema1d.StrategyKey, 50)
	if err != nil {
		return nil, err
	}
	items := make([]ConfigChange, len(rows))
	for i, r := range rows {
		var before, after map[string]any
		_ = json.Unmarshal(r.Before, &before)
		_ = json.Unmarshal(r.After, &after)
		items[i] = ConfigChange{Id: r.ID, Before: before, After: after, UserEmail: r.UserEmail, CreatedAt: r.CreatedAt.Time}
	}
	return GetEma1dConfigHistory200JSONResponse{Items: items}, nil
}

func nonNilSlice[T any](xs []T) []T {
	if xs == nil {
		return []T{}
	}
	return xs
}

// exchangeErr maps exchange failures of read calls to a 502 with the reason.
func exchangeErr(err error) error {
	var he *httpx.Error
	if errors.As(err, &he) {
		return err
	}
	return &httpx.Error{Status: 502, Code: "exchange_error", Message: "Binance call failed: " + err.Error(), Err: err}
}
