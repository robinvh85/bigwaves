import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { registerEventInvalidation } from '@/app/event-stream'
import { api, unwrap } from '@/shared/api/client'
import type { components } from '@/shared/api/schema'

export type EmaCrossPair = components['schemas']['EmaCrossPair']
export type EmaCrossAlert = components['schemas']['EmaCrossAlert']
export type EmaCrossState = components['schemas']['EmaCrossState']
export type EmaCrossConfig = components['schemas']['EmaCrossConfig']

/** The D1 check job (see backend emacross.Jobs). */
export const CHECK_JOB = 'emacross.check_d1'

export type AlertFilter = { pairId?: number }

export const emaCrossKeys = {
  all: ['emacross'] as const,
  pairs: () => [...emaCrossKeys.all, 'pairs'] as const,
  alerts: (f: AlertFilter) => [...emaCrossKeys.all, 'alerts', f] as const,
  config: () => [...emaCrossKeys.all, 'config'] as const,
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

function useInvalidate() {
  const qc = useQueryClient()
  return () => qc.invalidateQueries({ queryKey: emaCrossKeys.all })
}

export function useWatchPair() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: ({ pairId, watched }: { pairId: number; watched: boolean }) =>
      unwrap(api.PUT('/ema-cross/pairs/{id}', { params: { path: { id: pairId } }, body: { watched } })),
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
