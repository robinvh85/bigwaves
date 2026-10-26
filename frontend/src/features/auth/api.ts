import { queryOptions } from '@tanstack/react-query'

import { api, unwrap } from '@/shared/api/client'

export const authKeys = {
  me: ['auth', 'me'] as const,
}

export const meQuery = queryOptions({
  queryKey: authKeys.me,
  queryFn: () => unwrap(api.GET('/me')),
  staleTime: 5 * 60_000,
  retry: false,
})

/** Login is a full-page redirect through Auth0, handled by the Go server. */
export function login(redirectTo?: string) {
  const q = redirectTo ? `?redirect=${encodeURIComponent(redirectTo)}` : ''
  window.location.assign(`/auth/login${q}`)
}

export async function logout() {
  await fetch('/auth/logout', { method: 'POST', credentials: 'same-origin' })
  window.location.assign('/login')
}
