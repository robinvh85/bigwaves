import { queryOptions } from '@tanstack/react-query'

import { api, unwrap } from '@/shared/api/client'

export const systemQuery = queryOptions({
  queryKey: ['system'],
  queryFn: () => unwrap(api.GET('/system')),
  staleTime: 5 * 60_000,
})
