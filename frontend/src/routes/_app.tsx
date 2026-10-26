import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'

import { AppSidebar } from '@/app/layout/AppSidebar'
import { TopBar } from '@/app/layout/TopBar'
import { meQuery } from '@/features/auth/api'
import { ApiError } from '@/shared/api/errors'
import { SidebarInset, SidebarProvider } from '@/shared/ui/sidebar'

/** Authenticated layout: every route under _app requires a session. */
export const Route = createFileRoute('/_app')({
  beforeLoad: async ({ context, location }) => {
    try {
      await context.queryClient.query(meQuery)
    } catch (err) {
      if (err instanceof ApiError && err.isUnauthorized) {
        throw redirect({ to: '/login', search: { redirect: location.href } })
      }
      throw err
    }
  },
  component: AppLayout,
})

function AppLayout() {
  return (
    <SidebarProvider>
      <AppSidebar />
      <SidebarInset>
        <TopBar />
        <div className="flex-1 p-4 md:p-6">
          <Outlet />
        </div>
      </SidebarInset>
    </SidebarProvider>
  )
}
