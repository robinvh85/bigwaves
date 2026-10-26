package httpapi

import (
	"context"

	"bigwaves/internal/platform/httpx"
)

func (s *Server) GetMe(ctx context.Context, _ GetMeRequestObject) (GetMeResponseObject, error) {
	return nil, httpx.ErrUnauthorized
}
