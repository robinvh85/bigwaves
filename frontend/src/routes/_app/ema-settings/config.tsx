import { createFileRoute } from '@tanstack/react-router'

import { CampaignConfigCards } from '@/features/emasettings/components/ConfigCards'
import { PageHeader } from '@/shared/components/states'

export const Route = createFileRoute('/_app/ema-settings/config')({
  staticData: { title: 'Settings' },
  component: () => (
    <>
      <PageHeader title="EMA Settings" description="The scan switch and the D1 / H4 campaign parameters." />
      <div className="max-w-2xl space-y-4">
        <CampaignConfigCards />
      </div>
    </>
  ),
})
