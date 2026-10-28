package httpapi

import (
	"context"
	"encoding/json"
	"errors"

	"bigwaves/internal/auth"
	"bigwaves/internal/emasettings"
	"bigwaves/internal/platform/configstore"
	"bigwaves/internal/platform/httpx"
)

// strp turns an optional generated enum into an optional string.
func strp[T ~string](v *T) *string {
	if v == nil {
		return nil
	}
	s := string(*v)
	return &s
}

func (s *Server) ListEmaSettings(ctx context.Context, req ListEmaSettingsRequestObject) (ListEmaSettingsResponseObject, error) {
	rows, err := s.EMASets.List(ctx, deref(strp(req.Params.Status)))
	if err != nil {
		return nil, err
	}
	return convert[ListEmaSettings200JSONResponse](map[string]any{"items": rows})
}

func (s *Server) CreateEmaSetting(ctx context.Context, req CreateEmaSettingRequestObject) (CreateEmaSettingResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("missing body", nil)
	}
	b := req.Body
	row, err := s.EMASets.CreateSetting(ctx, emasettings.SettingInput{
		PairID: b.PairId, Budget: &b.Budget, TradeType: strp(b.TradeType), Period: strp(b.Period), CheckType: strp(b.CheckType),
		NextTradeType: strp(b.NextTradeType), TpPercent: b.TpPercent, SlPercent: b.SlPercent, Status: strp(b.Status), Note: b.Note,
	})
	if err != nil {
		return nil, err
	}
	return convert[CreateEmaSetting201JSONResponse](row)
}

func (s *Server) GetEmaSetting(ctx context.Context, req GetEmaSettingRequestObject) (GetEmaSettingResponseObject, error) {
	row, err := s.EMASets.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return convert[GetEmaSetting200JSONResponse](row)
}

func (s *Server) UpdateEmaSetting(ctx context.Context, req UpdateEmaSettingRequestObject) (UpdateEmaSettingResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("missing body", nil)
	}
	b := req.Body
	row, err := s.EMASets.UpdateSetting(ctx, req.Id, emasettings.SettingInput{
		Budget: b.Budget, TradeType: strp(b.TradeType), Period: strp(b.Period), CheckType: strp(b.CheckType),
		NextTradeType: strp(b.NextTradeType), TpPercent: b.TpPercent, SlPercent: b.SlPercent, Status: strp(b.Status), Note: b.Note,
	})
	if err != nil {
		return nil, err
	}
	return convert[UpdateEmaSetting200JSONResponse](row)
}

func (s *Server) DeleteEmaSetting(ctx context.Context, req DeleteEmaSettingRequestObject) (DeleteEmaSettingResponseObject, error) {
	if err := s.EMASets.DeleteSetting(ctx, req.Id); err != nil {
		return nil, err
	}
	return DeleteEmaSetting204Response{}, nil
}

func (s *Server) CloseEmaSetting(ctx context.Context, req CloseEmaSettingRequestObject) (CloseEmaSettingResponseObject, error) {
	row, err := s.EMASets.CloseSetting(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return convert[CloseEmaSetting200JSONResponse](row)
}

func (s *Server) CreateEmaSettingTrade(ctx context.Context, req CreateEmaSettingTradeRequestObject) (CreateEmaSettingTradeResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("missing body", nil)
	}
	b := req.Body
	t, err := s.EMASets.CreateTrade(ctx, req.Id, emasettings.TradeInput{
		Side: string(b.Side), Status: deref(strp(b.Status)), Amount: b.Amount, EntryPrice: b.EntryPrice,
		TargetPrice: b.TargetPrice, SlPrice: b.SlPrice, OpenPrice: b.OpenPrice, OpenAt: b.OpenAt,
	})
	if err != nil {
		return nil, err
	}
	return convert[CreateEmaSettingTrade201JSONResponse](t)
}

func (s *Server) UpdateEmaSettingTrade(ctx context.Context, req UpdateEmaSettingTradeRequestObject) (UpdateEmaSettingTradeResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("missing body", nil)
	}
	b := req.Body
	t, err := s.EMASets.UpdateTrade(ctx, req.Id, emasettings.TradeUpdate{
		Side: strp(b.Side), Amount: b.Amount, EntryPrice: b.EntryPrice, TargetPrice: b.TargetPrice, SlPrice: b.SlPrice,
		Status: strp(b.Status), ClosePrice: b.ClosePrice,
	})
	if err != nil {
		return nil, err
	}
	return convert[UpdateEmaSettingTrade200JSONResponse](t)
}

func (s *Server) DeleteEmaSettingTrade(ctx context.Context, req DeleteEmaSettingTradeRequestObject) (DeleteEmaSettingTradeResponseObject, error) {
	if err := s.EMASets.DeleteTrade(ctx, req.Id); err != nil {
		return nil, err
	}
	return DeleteEmaSettingTrade204Response{}, nil
}

func (s *Server) ForceOpenEmaSettingTrade(ctx context.Context, req ForceOpenEmaSettingTradeRequestObject) (ForceOpenEmaSettingTradeResponseObject, error) {
	t, err := s.EMASets.ForceOpen(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return convert[ForceOpenEmaSettingTrade200JSONResponse](t)
}

func (s *Server) ForceCloseEmaSettingTrade(ctx context.Context, req ForceCloseEmaSettingTradeRequestObject) (ForceCloseEmaSettingTradeResponseObject, error) {
	t, err := s.EMASets.ForceClose(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return convert[ForceCloseEmaSettingTrade200JSONResponse](t)
}

func (s *Server) PartialCloseEmaSettingTrade(ctx context.Context, req PartialCloseEmaSettingTradeRequestObject) (PartialCloseEmaSettingTradeResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("missing body", nil)
	}
	t, err := s.EMASets.PartialClose(ctx, req.Id, req.Body.Amount)
	if err != nil {
		return nil, err
	}
	return convert[PartialCloseEmaSettingTrade200JSONResponse](t)
}

func (s *Server) SplitEmaSettingTrade(ctx context.Context, req SplitEmaSettingTradeRequestObject) (SplitEmaSettingTradeResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("missing body", nil)
	}
	t, err := s.EMASets.Split(ctx, req.Id, req.Body.Amount)
	if err != nil {
		return nil, err
	}
	return convert[SplitEmaSettingTrade201JSONResponse](t)
}

func (s *Server) emaSettingsEnvelope(ctx context.Context) (EmaSettingsConfigEnvelope, error) {
	c, v, err := s.EMASets.Config(ctx)
	if err != nil {
		return EmaSettingsConfigEnvelope{}, err
	}
	return convert[EmaSettingsConfigEnvelope](map[string]any{"config": c, "version": v})
}

func (s *Server) GetEmaSettingsConfig(ctx context.Context, _ GetEmaSettingsConfigRequestObject) (GetEmaSettingsConfigResponseObject, error) {
	env, err := s.emaSettingsEnvelope(ctx)
	return GetEmaSettingsConfig200JSONResponse(env), err
}

func (s *Server) UpdateEmaSettingsConfig(ctx context.Context, req UpdateEmaSettingsConfigRequestObject) (UpdateEmaSettingsConfigResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.Validation("missing body", nil)
	}
	c, err := convert[emasettings.Config](req.Body.Config)
	if err != nil {
		return nil, httpx.Validation(err.Error(), nil)
	}
	u, _ := auth.UserFrom(ctx)
	if _, err := s.EMASets.SaveConfig(ctx, req.Body.Version, c, u.Email); err != nil {
		if errors.Is(err, configstore.ErrVersionConflict) {
			return nil, httpx.Conflict("version_conflict", "the settings were just changed by someone else; reload and try again")
		}
		return nil, err
	}
	env, err := s.emaSettingsEnvelope(ctx)
	return UpdateEmaSettingsConfig200JSONResponse(env), err
}

func (s *Server) GetEmaSettingsConfigHistory(ctx context.Context, _ GetEmaSettingsConfigHistoryRequestObject) (GetEmaSettingsConfigHistoryResponseObject, error) {
	rows, err := s.Configs.History(ctx, emasettings.StrategyKey, 50)
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
	return GetEmaSettingsConfigHistory200JSONResponse{Items: items}, nil
}
