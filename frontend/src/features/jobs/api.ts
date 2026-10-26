import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/shared/api/client'
import type { components } from '@/shared/api/schema'

export type JobStatus = components['schemas']['JobStatus']
export type JobRun = components['schemas']['JobRun']

export const jobKeys = {
  all: ['jobs'] as const,
  list: () => [...jobKeys.all, 'list'] as const,
  runs: (name: string) => [...jobKeys.all, 'runs', name] as const,
}

export const jobsQuery = queryOptions({
  queryKey: jobKeys.list(),
  queryFn: () => unwrap(api.GET('/jobs')),
  select: (d) => d.items,
  // A running job finishes by an SSE event; poll slowly as a fallback.
  refetchInterval: 60_000,
})

export const jobRunsQuery = (name: string) =>
  queryOptions({
    queryKey: jobKeys.runs(name),
    queryFn: () => unwrap(api.GET('/jobs/{name}/runs', { params: { path: { name }, query: { limit: 20 } } })),
    select: (d) => d.items,
  })

export function useRunJob() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (name: string) => unwrap(api.POST('/jobs/{name}/run', { params: { path: { name } } })),
    onSuccess: () => qc.invalidateQueries({ queryKey: jobKeys.all }),
    meta: { success: 'Job started' },
  })
}
