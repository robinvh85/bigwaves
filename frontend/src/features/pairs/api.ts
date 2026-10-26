import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/shared/api/client'
import type { components } from '@/shared/api/schema'

export type Pair = components['schemas']['Pair']
export type PairStatus = components['schemas']['PairStatus']

export const pairKeys = {
  all: ['pairs'] as const,
  list: (f: { status?: PairStatus }) => [...pairKeys.all, 'list', f] as const,
  detail: (id: number) => [...pairKeys.all, 'detail', id] as const,
}

export const pairsQuery = (f: { status?: PairStatus } = {}) =>
  queryOptions({
    queryKey: pairKeys.list(f),
    queryFn: () => unwrap(api.GET('/pairs', { params: { query: { status: f.status } } })),
    select: (d) => d.items,
  })

export const pairQuery = (id: number) =>
  queryOptions({
    queryKey: pairKeys.detail(id),
    queryFn: () => unwrap(api.GET('/pairs/{id}', { params: { path: { id } } })),
  })

export function useCreatePair() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: components['schemas']['CreatePairRequest']) => unwrap(api.POST('/pairs', { body })),
    onSuccess: () => qc.invalidateQueries({ queryKey: pairKeys.all }),
    meta: { silent: true },
  })
}

export function useUpdatePair(id: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: components['schemas']['UpdatePairRequest']) =>
      unwrap(api.PATCH('/pairs/{id}', { params: { path: { id } }, body })),
    onSuccess: (p) => {
      qc.setQueryData(pairKeys.detail(id), p)
      return qc.invalidateQueries({ queryKey: pairKeys.all })
    },
    meta: { success: 'Saved' },
  })
}

export function useDeletePair() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => unwrap(api.DELETE('/pairs/{id}', { params: { path: { id } } })),
    onSuccess: () => qc.invalidateQueries({ queryKey: pairKeys.all }),
    meta: { success: 'Pair deleted' },
  })
}

export function useUpdateCoinDescription(pairId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ coinId, description }: { coinId: number; description: string }) =>
      unwrap(api.PATCH('/coins/{id}', { params: { path: { id: coinId } }, body: { description } })),
    onSuccess: () =>
      Promise.all([
        qc.invalidateQueries({ queryKey: pairKeys.detail(pairId) }),
        qc.invalidateQueries({ queryKey: pairKeys.all }),
      ]),
    meta: { success: 'Notes saved' },
  })
}
