import { createFileRoute } from '@tanstack/react-router'

import { ConfigForm } from '@/features/ema1d/components/ConfigForm'
import { ConfigHistory } from '@/features/ema1d/components/ConfigHistory'
import { StrategySwitches } from '@/features/ema1d/components/StrategySwitches'
import { PageHeader } from '@/shared/components/states'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/shared/ui/tabs'

export const Route = createFileRoute('/_app/ema1d/config')({
  staticData: { title: 'Cấu hình' },
  component: () => (
    <>
      <PageHeader
        title="Cấu hình EMA1D"
        description="Mọi thay đổi được ghi lại kèm người sửa và giá trị cũ."
      />
      <div className="mb-4 rounded-lg border p-3">
        <StrategySwitches />
      </div>
      <Tabs defaultValue="form">
        <TabsList>
          <TabsTrigger value="form">Tham số</TabsTrigger>
          <TabsTrigger value="history">Lịch sử thay đổi</TabsTrigger>
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
