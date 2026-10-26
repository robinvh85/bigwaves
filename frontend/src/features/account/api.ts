import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/shared/api/client'

export const accountKeys = {
  all: ['account'] as const,
  balances: () => [...accountKeys.all, 'balances'] as const,
}

export const balancesQuery = queryOptions({
  queryKey: accountKeys.balances(),
  queryFn: () => unwrap(api.GET('/account/balances')),
  staleTime: 30_000,
})

export type AccountAction = 'borrow' | 'repay' | 'spotToCross' | 'crossToSpot'

export function useAccountAction() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ action, amount }: { action: AccountAction; amount: string }) => {
      switch (action) {
        case 'borrow':
          return unwrap(api.POST('/account/borrow', { body: { amount } }))
        case 'repay':
          return unwrap(api.POST('/account/repay', { body: { amount } }))
        case 'spotToCross':
          return unwrap(api.POST('/account/transfer', { body: { direction: 'SPOT_TO_CROSS', amount } }))
        case 'crossToSpot':
          return unwrap(api.POST('/account/transfer', { body: { direction: 'CROSS_TO_SPOT', amount } }))
      }
    },
    onSettled: () => qc.invalidateQueries({ queryKey: accountKeys.all }),
    meta: { success: 'Done' },
  })
}
