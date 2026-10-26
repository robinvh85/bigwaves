import { MutationCache, QueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'

import { ApiError, errorMessage } from '@/shared/api/errors'

declare module '@tanstack/react-query' {
  interface Register {
    mutationMeta: {
      /** Skip the default error toast (the caller shows the error itself). */
      silent?: boolean
      /** Toast shown on success. */
      success?: string
    }
  }
}

export const queryClient = new QueryClient({
  mutationCache: new MutationCache({
    onError: (err, _vars, _ctx, mutation) => {
      if (mutation.meta?.silent) return
      if (err instanceof ApiError && err.isUnauthorized) return
      toast.error(errorMessage(err))
    },
    onSuccess: (_data, _vars, _ctx, mutation) => {
      if (mutation.meta?.success) toast.success(mutation.meta.success)
    },
  }),
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
