import { createFileRoute } from '@tanstack/react-router'

import { ConfigForm } from '@/features/ema1d/components/ConfigForm'
import { ConfigHistory } from '@/features/ema1d/components/ConfigHistory'
import { StrategySwitches } from '@/features/ema1d/components/StrategySwitches'
import { PageHeader } from '@/shared/components/states'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/shared/ui/tabs'

export const Route = createFileRoute('/_app/ema1d/config')({
  staticData: { title: 'Settings' },
  component: () => (
    <>
      <PageHeader
        title="EMA1D settings"
        description="Every change is recorded with who made it and the previous value."
      />
      <div className="mb-4 rounded-lg border p-3">
        <StrategySwitches />
      </div>
      <Tabs defaultValue="form">
        <TabsList>
          <TabsTrigger value="form">Parameters</TabsTrigger>
          <TabsTrigger value="history">Change history</TabsTrigger>
        </TabsList>
        <TabsContent value="form" className="max-w-3xl">
          <ConfigForm />
        </TabsContent>
        <TabsContent value="history" className="max-w-3xl">
          <ConfigHistory />
        </TabsContent>
      </Tabs>
    </>
  ),
})
