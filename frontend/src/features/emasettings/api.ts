import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { registerEventInvalidation } from '@/app/event-stream'
import { api, unwrap } from '@/shared/api/client'
import type { components } from '@/shared/api/schema'

export type EmaSetting = components['schemas']['EmaSetting']
export type EmaSettingTrade = components['schemas']['EmaSettingTrade']
export type EmaSettingInput = components['schemas']['EmaSettingInput']
export type EmaSettingTradeInput = components['schemas']['EmaSettingTradeInput']
export type EmaSettingTradeUpdate = components['schemas']['EmaSettingTradeUpdate']
export type EmaSettingsConfig = components['schemas']['EmaSettingsConfig']
export type EmaSettingStatus = components['schemas']['EmaSettingStatus']

export const TRADE_TYPES = ['BOTH', 'ONLY_LONG', 'ONLY_SHORT'] as const
export const PERIODS = ['D1', 'H4'] as const
export const CHECK_TYPES = ['CHECK_AT_OPEN', 'CHECK_AT_CLOSE', 'CHECK_AT_CURRENT'] as const

export const TRADE_TYPE_LABEL: Record<EmaSetting['tradeType'], string> = {
  BOTH: 'Long & short',
  ONLY_LONG: 'Long only',
  ONLY_SHORT: 'Short only',
}

export const CHECK_TYPE_LABEL: Record<EmaSetting['checkType'], string> = {
  CHECK_AT_OPEN: 'At candle open (current price)',
  CHECK_AT_CLOSE: 'At candle close (closed candle)',
  CHECK_AT_CURRENT: 'Every scan (current price)',
}

export const CLOSE_REASON_LABEL: Record<string, string> = {
  SL: 'Stop loss',
  TARGET: 'Target',
  EMA_REVERSE: 'EMA cross back',
  SETTING_TP: 'Campaign take profit',
  SETTING_SL: 'Campaign stop loss',
  MANUAL: 'By hand',
  CANCELLED: 'Cancelled',
}

export const emaSettingsKeys = {
  all: ['emasettings'] as const,
  list: (status?: EmaSettingStatus) => [...emaSettingsKeys.all, 'list', status ?? 'all'] as const,
  detail: (id: number) => [...emaSettingsKeys.all, 'detail', id] as const,
  config: () => [...emaSettingsKeys.all, 'config'] as const,
  history: () => [...emaSettingsKeys.all, 'config', 'history'] as const,
}

registerEventInvalidation('emasettings.updated', [emaSettingsKeys.all])
registerEventInvalidation('prices.updated', [emaSettingsKeys.all])

export const emaSettingsListQuery = (status?: EmaSettingStatus) =>
  queryOptions({
    queryKey: emaSettingsKeys.list(status),
    queryFn: () => unwrap(api.GET('/ema-settings/settings', { params: { query: { status } } })),
    select: (d) => d.items,
  })

export const emaSettingDetailQuery = (id: number) =>
  queryOptions({
    queryKey: emaSettingsKeys.detail(id),
    queryFn: () => unwrap(api.GET('/ema-settings/settings/{id}', { params: { path: { id } } })),
  })

export const emaSettingsConfigQuery = queryOptions({
  queryKey: emaSettingsKeys.config(),
  queryFn: () => unwrap(api.GET('/ema-settings/config')),
  staleTime: 0,
})

export const emaSettingsHistoryQuery = queryOptions({
  queryKey: emaSettingsKeys.history(),
  queryFn: () => unwrap(api.GET('/ema-settings/config/history')),
  select: (d) => d.items,
})

function useInvalidate() {
  const qc = useQueryClient()
  return () => qc.invalidateQueries({ queryKey: emaSettingsKeys.all })
}

export function useSaveSetting() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: ({ id, body }: { id?: number; body: EmaSettingInput & { pairId?: number } }) =>
      id
        ? unwrap(api.PATCH('/ema-settings/settings/{id}', { params: { path: { id } }, body }))
        : unwrap(
            api.POST('/ema-settings/settings', {
              body: { ...body, pairId: body.pairId ?? 0, budget: body.budget ?? '0' },
            }),
          ),
    onSuccess: invalidate,
    meta: { success: 'Saved' },
  })
}

export function useSettingAction() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: ({ id, action }: { id: number; action: 'close' | 'delete' }) => {
      const params = { params: { path: { id } } }
      if (action === 'close') return unwrap(api.POST('/ema-settings/settings/{id}/close', params))
      return unwrap(api.DELETE('/ema-settings/settings/{id}', params))
    },
    onSuccess: invalidate,
    meta: { success: 'Done' },
  })
}

export function useCreateTrade(settingId: number) {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: (body: EmaSettingTradeInput) =>
      unwrap(api.POST('/ema-settings/settings/{id}/trades', { params: { path: { id: settingId } }, body })),
    onSuccess: invalidate,
    meta: { success: 'Trade added' },
  })
}

export function useUpdateTrade() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: EmaSettingTradeUpdate }) =>
      unwrap(api.PATCH('/ema-settings/trades/{id}', { params: { path: { id } }, body })),
    onSuccess: invalidate,
    meta: { success: 'Trade saved' },
  })
}

export type TradeAction = 'force-open' | 'force-close' | 'delete' | 'partial-close' | 'split'

export function useTradeAction() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: ({ id, action, amount }: { id: number; action: TradeAction; amount?: string }) => {
      const params = { params: { path: { id } } }
      switch (action) {
        case 'force-open':
          return unwrap(api.POST('/ema-settings/trades/{id}/force-open', params))
        case 'force-close':
          return unwrap(api.POST('/ema-settings/trades/{id}/force-close', params))
        case 'partial-close':
          return unwrap(
            api.POST('/ema-settings/trades/{id}/partial-close', {
              ...params,
              body: { amount: amount ?? '0' },
            }),
          )
        case 'split':
          return unwrap(
            api.POST('/ema-settings/trades/{id}/split', { ...params, body: { amount: amount ?? '0' } }),
          )
        default:
          return unwrap(api.DELETE('/ema-settings/trades/{id}', params))
      }
    },
    onSuccess: invalidate,
    meta: { success: 'Done' },
  })
}

export function useSaveEmaSettingsConfig() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: (body: { version: number; config: EmaSettingsConfig }) =>
      unwrap(api.PATCH('/ema-settings/config', { body })),
    onSuccess: invalidate,
    meta: { success: 'Settings saved' },
  })
}
