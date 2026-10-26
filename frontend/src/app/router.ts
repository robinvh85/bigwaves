import { createRouter } from '@tanstack/react-router'

import { queryClient } from '@/app/query-client'
import { routeTree } from '@/routeTree.gen'
import { setUnauthorizedHandler } from '@/shared/api/client'

export const router = createRouter({
  routeTree,
  context: { queryClient },
  defaultPreload: 'intent',
  // Query data is cached by TanStack Query; the router should not cache loader results itself.
  defaultPreloadStaleTime: 0,
  scrollRestoration: true,
})

// A 401 anywhere (expired session) drops cached data and sends the user to the login page.
setUnauthorizedHandler(() => {
  if (router.state.location.pathname === '/login') return
  queryClient.clear()
  void router.navigate({ to: '/login', search: { redirect: router.state.location.href } })
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
  interface StaticDataRouteOption {
    /** Shown in the breadcrumb and the document title. */
    title?: string
  }
}
