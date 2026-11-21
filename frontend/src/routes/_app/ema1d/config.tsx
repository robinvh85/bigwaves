import { createFileRoute } from '@tanstack/react-router'

import { ConfigForm } from '@/features/ema1d/components/ConfigForm'
import { StrategySwitches } from '@/features/ema1d/components/StrategySwitches'
import { PageHeader } from '@/shared/components/states'

export const Route = createFileRoute('/_app/ema1d/config')({
  staticData: { title: 'Settings' },
  component: () => (
    <>
      <PageHeader
        title="EMA1D settings"
        description="Changes take effect from the next sync."
      />
      <div className="mb-4 rounded-lg border p-3">
        <StrategySwitches />
      </div>
      <div className="max-w-3xl">
        <ConfigForm />
      </div>
    </>
  ),
})
