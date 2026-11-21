package httpapi

import (
	"context"
	"errors"

	"bigwaves/internal/auth"
	"bigwaves/internal/emacross"
	"bigwaves/internal/platform/configstore"
	"bigwaves/internal/platform/httpx"
)

func (s *Server) ListEmaCrossPairs(ctx context.Context, _ ListEmaCrossPairsRequestObject) (ListEmaCrossPairsResponseObject, error) {
	rows, err := s.EMACross.List(ctx)
	if err != nil {
		return nil, err
	}
	return convert[ListEmaCrossPairs200JSONResponse](map[string]any{"items": rows})
}

func (s *Server) WatchEmaCrossPair(ctx context.Context, req WatchEmaCrossPairRequestObject) (WatchEmaCrossPairResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("missing body", nil)
	}
	row, err := s.EMACross.Watch(ctx, req.Id, req.Body.Watched)
	if err != nil {
		return nil, err
	}
	return convert[WatchEmaCrossPair200JSONResponse](row)
}

func (s *Server) ListEmaCrossAlerts(ctx context.Context, req ListEmaCrossAlertsRequestObject) (ListEmaCrossAlertsResponseObject, error) {
	f := emacross.AlertFilter{PairID: deref(req.Params.PairId), Limit: int64(deref(req.Params.Limit))}
	rows, err := s.EMACross.Alerts(ctx, f)
	if err != nil {
		return nil, err
	}
	return convert[ListEmaCrossAlerts200JSONResponse](map[string]any{"items": rows})
}

func (s *Server) emaCrossEnvelope(ctx context.Context) (EmaCrossConfigEnvelope, error) {
	c, v, err := s.EMACross.Config(ctx)
	if err != nil {
		return EmaCrossConfigEnvelope{}, err
	}
	return convert[EmaCrossConfigEnvelope](map[string]any{"config": c, "version": v})
}

func (s *Server) GetEmaCrossConfig(ctx context.Context, _ GetEmaCrossConfigRequestObject) (GetEmaCrossConfigResponseObject, error) {
	env, err := s.emaCrossEnvelope(ctx)
	return GetEmaCrossConfig200JSONResponse(env), err
}

func (s *Server) UpdateEmaCrossConfig(ctx context.Context, req UpdateEmaCrossConfigRequestObject) (UpdateEmaCrossConfigResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("missing body", nil)
	}
	c, err := convert[emacross.Config](req.Body.Config)
	if err != nil {
		return nil, httpx.Validation(err.Error(), nil)
	}
	u, _ := auth.UserFrom(ctx)
	if _, err := s.EMACross.SaveConfig(ctx, req.Body.Version, c, u.Email); err != nil {
		if errors.Is(err, configstore.ErrVersionConflict) {
			return nil, httpx.Conflict("version_conflict", "the settings were just changed by someone else; reload and try again")
		}
		return nil, err
	}
	env, err := s.emaCrossEnvelope(ctx)
	return UpdateEmaCrossConfig200JSONResponse(env), err
}
