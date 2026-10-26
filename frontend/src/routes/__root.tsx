import type { QueryClient } from '@tanstack/react-query'
import { createRootRouteWithContext, Outlet } from '@tanstack/react-router'

import { useApplyTheme } from '@/app/theme'
import { ErrorState } from '@/shared/components/states'
import { Toaster } from '@/shared/ui/sonner'
import { TooltipProvider } from '@/shared/ui/tooltip'

export type RouterContext = { queryClient: QueryClient }

export const Route = createRootRouteWithContext<RouterContext>()({
  component: Root,
  errorComponent: ({ error, reset }) => (
    <div className="p-6">
      <ErrorState error={error} onRetry={reset} />
    </div>
  ),
  notFoundComponent: () => (
    <div className="p-6">
      <ErrorState title="Không tìm thấy trang" error={null} />
    </div>
  ),
})

function Root() {
  useApplyTheme()
  return (
    <TooltipProvider delayDuration={300}>
      <Outlet />
      <Toaster richColors closeButton />
    </TooltipProvider>
  )
}
