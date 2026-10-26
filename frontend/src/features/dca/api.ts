import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { registerEventInvalidation } from '@/app/event-stream'
import { api, unwrap } from '@/shared/api/client'
import type { components } from '@/shared/api/schema'

export type DcaSetting = components['schemas']['DcaSetting']
export type DcaTrade = components['schemas']['DcaTrade']
export type DcaTradeInput = components['schemas']['DcaTradeInput']
export type DcaConfig = components['schemas']['DcaConfig']

export const dcaKeys = {
  all: ['dca'] as const,
  list: () => [...dcaKeys.all, 'list'] as const,
  detail: (id: number) => [...dcaKeys.all, 'detail', id] as const,
  config: () => [...dcaKeys.all, 'config'] as const,
  history: () => [...dcaKeys.all, 'config', 'history'] as const,
}

registerEventInvalidation('dca.updated', [dcaKeys.all])
registerEventInvalidation('prices.updated', [dcaKeys.all])

export const dcaListQuery = queryOptions({
  queryKey: dcaKeys.list(),
  queryFn: () => unwrap(api.GET('/dca/settings')),
  select: (d) => d.items,
})

export const dcaDetailQuery = (id: number) =>
  queryOptions({
    queryKey: dcaKeys.detail(id),
    queryFn: () => unwrap(api.GET('/dca/settings/{id}', { params: { path: { id } } })),
  })

export const dcaConfigQuery = queryOptions({
  queryKey: dcaKeys.config(),
  queryFn: () => unwrap(api.GET('/dca/config')),
  staleTime: 0,
})

export const dcaHistoryQuery = queryOptions({
  queryKey: dcaKeys.history(),
  queryFn: () => unwrap(api.GET('/dca/config/history')),
  select: (d) => d.items,
})

function useInvalidate() {
  const qc = useQueryClient()
  return () => qc.invalidateQueries({ queryKey: dcaKeys.all })
}

export function useCreateSetting() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: (body: { pairId: number; note?: string }) => unwrap(api.POST('/dca/settings', { body })),
    onSuccess: invalidate,
    meta: { success: 'Đã tạo ladder' },
  })
}

export function useUpdateSetting(id: number) {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: (body: { status?: DcaSetting['status']; note?: string }) =>
      unwrap(api.PATCH('/dca/settings/{id}', { params: { path: { id } }, body })),
    onSuccess: invalidate,
    meta: { success: 'Đã lưu' },
  })
}

export function useDeleteSetting() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: (id: number) => unwrap(api.DELETE('/dca/settings/{id}', { params: { path: { id } } })),
    onSuccess: invalidate,
    meta: { success: 'Đã xoá ladder' },
  })
}

export function useSaveTrade(settingId: number) {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: ({ id, body }: { id?: number; body: DcaTradeInput }) =>
      id
        ? unwrap(api.PATCH('/dca/trades/{id}', { params: { path: { id } }, body }))
        : unwrap(api.POST('/dca/settings/{id}/trades', { params: { path: { id: settingId } }, body })),
    onSuccess: invalidate,
    meta: { silent: true },
  })
}

export function useTradeAction() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: ({ id, action }: { id: number; action: 'buy' | 'sell' | 'delete' }) => {
      const params = { params: { path: { id } } }
      if (action === 'buy') return unwrap(api.POST('/dca/trades/{id}/buy', params))
      if (action === 'sell') return unwrap(api.POST('/dca/trades/{id}/sell', params))
      return unwrap(api.DELETE('/dca/trades/{id}', params))
    },
    onSuccess: invalidate,
    meta: { success: 'Đã thực hiện' },
  })
}

export function useSaveDcaConfig() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: (body: { version: number; config: DcaConfig }) => unwrap(api.PATCH('/dca/config', { body })),
    onSuccess: invalidate,
    meta: { success: 'Đã lưu cấu hình' },
  })
}
