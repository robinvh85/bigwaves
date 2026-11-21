import { createFileRoute, Outlet } from '@tanstack/react-router'

import { SECTION_TABS } from '@/app/nav'
import { SectionTabs } from '@/shared/components/section-tabs'

export const Route = createFileRoute('/_app/dca')({
  staticData: { title: 'DCA' },
  component: () => (
    <>
      <SectionTabs tabs={SECTION_TABS.dca} />
      <Outlet />
    </>
  ),
})
