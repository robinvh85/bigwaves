import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { registerEventInvalidation } from '@/app/event-stream'
import { api, unwrap } from '@/shared/api/client'
import type { components } from '@/shared/api/schema'

export type EmaCrossPair = components['schemas']['EmaCrossPair']
export type EmaCrossAlert = components['schemas']['EmaCrossAlert']
export type EmaCrossState = components['schemas']['EmaCrossState']
export type EmaCrossConfig = components['schemas']['EmaCrossConfig']
export type Timeframe = components['schemas']['EmaCrossTimeframe']
export type Watch = components['schemas']['EmaCrossWatchRequest']

export const TIMEFRAMES: Timeframe[] = ['H1', 'H4', 'D1']
export const watchKey = (tf: Timeframe) => tf.toLowerCase() as keyof Watch

/** Job that checks one timeframe (see backend emacross.Jobs). */
export const checkJob = (tf: Timeframe) => `emacross.check_${tf.toLowerCase()}`

export type AlertFilter = { pairId?: number; timeframe?: Timeframe }

export const emaCrossKeys = {
  all: ['emacross'] as const,
  pairs: () => [...emaCrossKeys.all, 'pairs'] as const,
  alerts: (f: AlertFilter) => [...emaCrossKeys.all, 'alerts', f] as const,
  config: () => [...emaCrossKeys.all, 'config'] as const,
  history: () => [...emaCrossKeys.all, 'config', 'history'] as const,
}

registerEventInvalidation('emacross.updated', [emaCrossKeys.all])

export const emaCrossPairsQuery = queryOptions({
  queryKey: emaCrossKeys.pairs(),
  queryFn: () => unwrap(api.GET('/ema-cross/pairs')),
  select: (d) => d.items,
})

export const emaCrossAlertsQuery = (f: AlertFilter) =>
  queryOptions({
    queryKey: emaCrossKeys.alerts(f),
    queryFn: () => unwrap(api.GET('/ema-cross/alerts', { params: { query: { ...f, limit: 500 } } })),
    select: (d) => d.items,
  })

export const emaCrossConfigQuery = queryOptions({
  queryKey: emaCrossKeys.config(),
  queryFn: () => unwrap(api.GET('/ema-cross/config')),
  staleTime: 0,
})

export const emaCrossHistoryQuery = queryOptions({
  queryKey: emaCrossKeys.history(),
  queryFn: () => unwrap(api.GET('/ema-cross/config/history')),
  select: (d) => d.items,
})

function useInvalidate() {
  const qc = useQueryClient()
  return () => qc.invalidateQueries({ queryKey: emaCrossKeys.all })
}

export function useWatchPair() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: ({ pairId, body }: { pairId: number; body: Watch }) =>
      unwrap(api.PUT('/ema-cross/pairs/{id}', { params: { path: { id: pairId } }, body })),
    onSuccess: invalidate,
    meta: { success: 'Saved' },
  })
}

export function useSaveEmaCrossConfig() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: (body: { version: number; config: EmaCrossConfig }) =>
      unwrap(api.PATCH('/ema-cross/config', { body })),
    onSuccess: invalidate,
    meta: { success: 'Settings saved' },
  })
}
