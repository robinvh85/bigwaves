package httpapi

import (
	"context"
	"encoding/json"
	"errors"

	"bigwaves/internal/auth"
	"bigwaves/internal/dca"
	"bigwaves/internal/platform/configstore"
	"bigwaves/internal/platform/httpx"
)

func (s *Server) ListDcaSettings(ctx context.Context, req ListDcaSettingsRequestObject) (ListDcaSettingsResponseObject, error) {
	status := ""
	if req.Params.Status != nil {
		status = string(*req.Params.Status)
	}
	rows, err := s.DCA.List(ctx, status)
	if err != nil {
		return nil, err
	}
	return convert[ListDcaSettings200JSONResponse](map[string]any{"items": rows})
}

func (s *Server) CreateDcaSetting(ctx context.Context, req CreateDcaSettingRequestObject) (CreateDcaSettingResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("missing body", nil)
	}
	row, err := s.DCA.CreateSetting(ctx, req.Body.PairId, deref(req.Body.Note))
	if err != nil {
		return nil, err
	}
	return convert[CreateDcaSetting201JSONResponse](row)
}

func (s *Server) GetDcaSetting(ctx context.Context, req GetDcaSettingRequestObject) (GetDcaSettingResponseObject, error) {
	row, err := s.DCA.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return convert[GetDcaSetting200JSONResponse](row)
}

func (s *Server) UpdateDcaSetting(ctx context.Context, req UpdateDcaSettingRequestObject) (UpdateDcaSettingResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("missing body", nil)
	}
	var status *string
	if req.Body.Status != nil {
		v := string(*req.Body.Status)
		status = &v
	}
	row, err := s.DCA.UpdateSetting(ctx, req.Id, status, req.Body.Note)
	if err != nil {
		return nil, err
	}
	return convert[UpdateDcaSetting200JSONResponse](row)
}

func (s *Server) DeleteDcaSetting(ctx context.Context, req DeleteDcaSettingRequestObject) (DeleteDcaSettingResponseObject, error) {
	if err := s.DCA.DeleteSetting(ctx, req.Id); err != nil {
		return nil, err
	}
	return DeleteDcaSetting204Response{}, nil
}

func tradeInput(b *DcaTradeInput) dca.TradeInput {
	return dca.TradeInput{BuyPrice: b.BuyPrice, SellPrice: b.SellPrice, Amount: b.Amount}
}

func (s *Server) CreateDcaTrade(ctx context.Context, req CreateDcaTradeRequestObject) (CreateDcaTradeResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("missing body", nil)
	}
	t, err := s.DCA.CreateTrade(ctx, req.Id, tradeInput(req.Body))
	if err != nil {
		return nil, err
	}
	return convert[CreateDcaTrade201JSONResponse](t)
}

func (s *Server) UpdateDcaTrade(ctx context.Context, req UpdateDcaTradeRequestObject) (UpdateDcaTradeResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("missing body", nil)
	}
	t, err := s.DCA.UpdateTrade(ctx, req.Id, tradeInput(req.Body))
	if err != nil {
		return nil, err
	}
	return convert[UpdateDcaTrade200JSONResponse](t)
}

func (s *Server) DeleteDcaTrade(ctx context.Context, req DeleteDcaTradeRequestObject) (DeleteDcaTradeResponseObject, error) {
	if err := s.DCA.DeleteTrade(ctx, req.Id); err != nil {
		return nil, err
	}
	return DeleteDcaTrade204Response{}, nil
}

func (s *Server) ForceBuyDcaTrade(ctx context.Context, req ForceBuyDcaTradeRequestObject) (ForceBuyDcaTradeResponseObject, error) {
	t, err := s.DCA.ForceBuy(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return convert[ForceBuyDcaTrade200JSONResponse](t)
}

func (s *Server) ForceSellDcaTrade(ctx context.Context, req ForceSellDcaTradeRequestObject) (ForceSellDcaTradeResponseObject, error) {
	t, err := s.DCA.ForceSell(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return convert[ForceSellDcaTrade200JSONResponse](t)
}

func (s *Server) dcaEnvelope(ctx context.Context) (DcaConfigEnvelope, error) {
	c, v, err := s.DCA.Config(ctx)
	if err != nil {
		return DcaConfigEnvelope{}, err
	}
	return convert[DcaConfigEnvelope](map[string]any{"config": c, "version": v})
}

func (s *Server) GetDcaConfig(ctx context.Context, _ GetDcaConfigRequestObject) (GetDcaConfigResponseObject, error) {
	env, err := s.dcaEnvelope(ctx)
	return GetDcaConfig200JSONResponse(env), err
}

func (s *Server) UpdateDcaConfig(ctx context.Context, req UpdateDcaConfigRequestObject) (UpdateDcaConfigResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("missing body", nil)
	}
	c, err := convert[dca.Config](req.Body.Config)
	if err != nil {
		return nil, httpx.Validation(err.Error(), nil)
	}
	u, _ := auth.UserFrom(ctx)
	if _, err := s.DCA.SaveConfig(ctx, req.Body.Version, c, u.Email); err != nil {
		if errors.Is(err, configstore.ErrVersionConflict) {
			return nil, httpx.Conflict("version_conflict", "the settings were just changed by someone else; reload and try again")
		}
		return nil, err
	}
	env, err := s.dcaEnvelope(ctx)
	return UpdateDcaConfig200JSONResponse(env), err
}

func (s *Server) GetDcaConfigHistory(ctx context.Context, _ GetDcaConfigHistoryRequestObject) (GetDcaConfigHistoryResponseObject, error) {
	rows, err := s.Configs.History(ctx, dca.StrategyKey, 50)
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
	return GetDcaConfigHistory200JSONResponse{Items: items}, nil
}
