package httpapi

import (
	"context"

	"bigwaves/internal/auth"
	"bigwaves/internal/platform/httpx"
)

func (s *Server) GetMe(ctx context.Context, _ GetMeRequestObject) (GetMeResponseObject, error) {
	u, ok := auth.UserFrom(ctx)
	if !ok {
		return nil, httpx.ErrUnauthorized
	}
	return GetMe200JSONResponse{Id: u.ID, Email: u.Email}, nil
}

func (s *Server) GetSystemStatus(_ context.Context, _ GetSystemStatusRequestObject) (GetSystemStatusResponseObject, error) {
	return GetSystemStatus200JSONResponse{
		Env:              s.Cfg.Env,
		TradingEnabled:   s.Cfg.TradingEnabled,
		SchedulerEnabled: s.Cfg.SchedulerEnabled,
	}, nil
}
