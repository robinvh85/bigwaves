# CLAUDE.md

bigwaves: rebuild of wavetrade (Rails + React) as a Go backend + SQLite + React frontend.
Plans live in `.claude/plans/` — read `00-overview.md` first. Business rules come from the old repo's
`~/Workspace/VH/wavetrades/wavetrade/docs/` (not from its outdated README/CLAUDE.md).

## Layout

- `api/openapi.yaml` — the contract. Change it first, then `make gen`.
- `backend/` — Go module `bigwaves`. `cmd/bigwaves` (server + scheduler), `cmd/bwctl` (operator CLI).
- `frontend/` — Vite + React 19 + TS strict, TanStack Router/Query/Table, shadcn/ui + Tailwind v4.

## Commands

```bash
make gen        # oapi-codegen (Go), sqlc, openapi-typescript (TS)
make dev        # Go on :8080 + Vite on :5173 (proxy /api, /auth)
make test       # go test -race + tsc + vitest
make lint       # golangci-lint + eslint
make build      # FE build embedded into one Go binary (backend/bin/bigwaves)
```

Single test: `cd backend && go test ./internal/ema1d/engine -run TestName`;
`cd frontend && npx vitest --run src/shared/lib/format.test.ts`.

## Rules

- Money/price/qty: `shopspring/decimal` in Go, TEXT in SQLite, string in JSON, `decimal.js-light` in TS.
  Never float, never `SUM()` over money columns in SQL.
- SQLite: one writer connection (`DB.Write`, BEGIN IMMEDIATE), readers on `DB.Read`. Never call
  Binance/Discord inside a DB transaction.
- Every exchange write call goes through the `TRADING_ENABLED` gate. Order retries belong to the
  queue, not the HTTP client; reuse orders by `clientOrderId` (prefix `bw_`).
- Handlers return `*httpx.Error` for expected failures; anything else becomes a 500.
- Frontend: server state only in TanStack Query (query key factories per feature), filters in URL
  search params, no `any`. Upstream shadcn code stays in `src/shared/ui`; our components go in
  `src/shared/components`. UI text is Vietnamese.
