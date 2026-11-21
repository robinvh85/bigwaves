import { createFileRoute, Outlet } from '@tanstack/react-router'

import { SECTION_TABS } from '@/app/nav'
import { SectionTabs } from '@/shared/components/section-tabs'

export const Route = createFileRoute('/_app/ema-settings')({
  staticData: { title: 'EMA Settings' },
  component: () => (
    <>
      <SectionTabs tabs={SECTION_TABS.emaSettings} />
      <Outlet />
    </>
  ),
})
