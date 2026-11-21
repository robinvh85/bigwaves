import { createFileRoute, Outlet } from '@tanstack/react-router'

import { SECTION_TABS } from '@/app/nav'
import { SectionTabs } from '@/shared/components/section-tabs'

export const Route = createFileRoute('/_app/ema1d')({
  staticData: { title: 'EMA1D' },
  component: () => (
    <>
      <SectionTabs tabs={SECTION_TABS.ema1d} />
      <Outlet />
    </>
  ),
})
