import { QueryClient } from '@tanstack/react-query'

import { ApiError } from '@/shared/api/errors'

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 15_000,
      // Do not retry client errors (4xx); retry network and 5xx errors twice.
      retry: (failureCount, err) => {
        if (err instanceof ApiError && err.status < 500) return false
        return failureCount < 2
      },
      refetchOnWindowFocus: true,
    },
    mutations: {
      // Mutations place real orders: never retry them automatically.
      retry: false,
    },
  },
})
