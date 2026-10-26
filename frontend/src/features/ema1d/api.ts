import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { registerEventInvalidation } from '@/app/event-stream'
import { api, unwrap } from '@/shared/api/client'
import type { components } from '@/shared/api/schema'

type S = components['schemas']
export type Ema1dStatus = S['Ema1dStatus']
export type Ema1dPosition = S['Ema1dPosition']
export type Ema1dTodo = S['Ema1dTodo']
export type Ema1dPairRow = S['Ema1dPairRow']
export type Ema1dPairUpdate = S['Ema1dPairUpdate']
export type Ema1dConfig = S['Ema1dConfig']
export type Ema1dConfigEnvelope = S['Ema1dConfigEnvelope']
export type Ema1dManualTodoRequest = S['Ema1dManualTodoRequest']
export type Ema1dSignalRow = S['Ema1dSignalRow']
export type Ema1dDailyCheck = S['Ema1dDailyCheck']
export type TradesFilter = {
  status?: 'open' | 'closed'
  coin?: string
  side?: 'LONG' | 'SHORT'
  month?: string
}

export const ema1dKeys = {
  all: ['ema1d'] as const,
  status: () => [...ema1dKeys.all, 'status'] as const,
  pairs: () => [...ema1dKeys.all, 'pairs'] as const,
  todos: (history: boolean) => [...ema1dKeys.all, 'todos', { history }] as const,
  trades: (f: TradesFilter) => [...ema1dKeys.all, 'trades', f] as const,
  equity: (days: number) => [...ema1dKeys.all, 'equity', days] as const,
  signals: () => [...ema1dKeys.all, 'signals'] as const,
  dailyChecks: () => [...ema1dKeys.all, 'daily-checks'] as const,
  config: () => [...ema1dKeys.all, 'config'] as const,
  configHistory: () => [...ema1dKeys.all, 'config', 'history'] as const,
}

// The queue, the monitor and the daily check publish "ema1d.updated": refresh everything EMA1D.
registerEventInvalidation('ema1d.updated', [ema1dKeys.all])

export const statusQuery = queryOptions({
  queryKey: ema1dKeys.status(),
  queryFn: () => unwrap(api.GET('/ema1d/status')),
  refetchInterval: 60_000,
})

export const pairsQuery = queryOptions({
  queryKey: ema1dKeys.pairs(),
  queryFn: () => unwrap(api.GET('/ema1d/pairs')),
  select: (d) => d.items,
})

export const todosQuery = (history: boolean) =>
  queryOptions({
    queryKey: ema1dKeys.todos(history),
    queryFn: () => unwrap(api.GET('/ema1d/todos', { params: { query: { history } } })),
    select: (d) => d.items,
  })

export const tradesQuery = (f: TradesFilter) =>
  queryOptions({
    queryKey: ema1dKeys.trades(f),
    queryFn: () => unwrap(api.GET('/ema1d/trades', { params: { query: f } })),
  })

export const equityQuery = (days: number) =>
  queryOptions({
    queryKey: ema1dKeys.equity(days),
    queryFn: () => unwrap(api.GET('/ema1d/equity', { params: { query: { days } } })),
    select: (d) => d.items,
  })

export const signalsQuery = queryOptions({
  queryKey: ema1dKeys.signals(),
  queryFn: () => unwrap(api.GET('/ema1d/signals')),
  staleTime: 5 * 60_000,
})

export const dailyChecksQuery = queryOptions({
  queryKey: ema1dKeys.dailyChecks(),
  queryFn: () => unwrap(api.GET('/ema1d/daily-checks', { params: { query: { limit: 30 } } })),
  select: (d) => d.items,
})

export const configQuery = queryOptions({
  queryKey: ema1dKeys.config(),
  queryFn: () => unwrap(api.GET('/ema1d/config')),
  staleTime: 0,
})

export const configHistoryQuery = queryOptions({
  queryKey: ema1dKeys.configHistory(),
  queryFn: () => unwrap(api.GET('/ema1d/config/history')),
  select: (d) => d.items,
})

function useInvalidateAll() {
  const qc = useQueryClient()
  return () => qc.invalidateQueries({ queryKey: ema1dKeys.all })
}

export function useSaveConfig() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: (body: { version: number; config: Ema1dConfig }) =>
      unwrap(api.PATCH('/ema1d/config', { body })),
    onSuccess: invalidate,
    meta: { success: 'Đã lưu cấu hình' },
  })
}

export function useUpdatePair(id: number) {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: (body: Ema1dPairUpdate) =>
      unwrap(api.PATCH('/ema1d/pairs/{id}', { params: { path: { id } }, body })),
    onSuccess: invalidate,
    meta: { success: 'Đã lưu' },
  })
}

export function useCheckPairs() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: (ids?: number[]) => unwrap(api.POST('/ema1d/pairs/check', { body: { ids } })),
    onSuccess: invalidate,
  })
}

export function useCreateTodo() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: (body: Ema1dManualTodoRequest) => unwrap(api.POST('/ema1d/todos', { body })),
    onSuccess: invalidate,
    meta: { silent: true },
  })
}

export function useRetryTodo() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: (id: number) => unwrap(api.POST('/ema1d/todos/{id}/retry', { params: { path: { id } } })),
    onSuccess: invalidate,
    meta: { success: 'Đã đưa lại vào hàng đợi' },
  })
}

export function useCancelTodo() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: (id: number) => unwrap(api.POST('/ema1d/todos/{id}/cancel', { params: { path: { id } } })),
    onSuccess: invalidate,
    meta: { success: 'Đã huỷ' },
  })
}
