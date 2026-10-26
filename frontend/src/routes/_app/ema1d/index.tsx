import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'

import { Ema1dAlerts } from '@/features/ema1d/components/Alerts'
import { Ema1dKpis } from '@/features/ema1d/components/Kpis'
import { LastSyncCard } from '@/features/ema1d/components/LastSyncCard'
import { ManualTodoDialog } from '@/features/ema1d/components/ManualTodoDialog'
import { openPositionColumns } from '@/features/ema1d/components/positionColumns'
import { StrategySwitches } from '@/features/ema1d/components/StrategySwitches'
import { TodoTable } from '@/features/ema1d/components/TodoTable'
import { statusQuery } from '@/features/ema1d/api'
import { DataTable } from '@/shared/components/data-table'
import { EmptyState, ErrorState, PageHeader, SkeletonRows } from '@/shared/components/states'
import { Button } from '@/shared/ui/button'

export const Route = createFileRoute('/_app/ema1d/')({
  staticData: { title: 'Bảng điều khiển' },
  component: Dashboard,
})

function Dashboard() {
  const { data: status, error, isLoading, refetch } = useQuery(statusQuery)

  return (
    <>
      <PageHeader
        title="EMA1D-EXIT"
        description="Bám xu hướng nến 1D trên rổ ~20 coin, cross margin. Không stop loss: chỉ đóng khi có tín hiệu thoát."
        actions={<ManualTodoDialog />}
      />
      <div className="mb-4 rounded-lg border p-3">
        <StrategySwitches />
      </div>
      {error ? (
        <ErrorState error={error} onRetry={() => void refetch()} />
      ) : isLoading || !status ? (
        <SkeletonRows rows={8} />
      ) : (
        <>
          <Ema1dAlerts status={status} />
          <Ema1dKpis status={status} />
          <section className="mb-6">
            <div className="mb-2 flex items-center justify-between">
              <h2 className="font-medium">Lệnh đang mở</h2>
              <Button variant="link" size="sm" asChild>
                <Link to="/ema1d/trades">Lịch sử lệnh →</Link>
              </Button>
            </div>
            <DataTable
              columns={openPositionColumns}
              data={status.openPositions}
              getRowId={(p) => String(p.id)}
              sorting={[{ id: 'pnl', desc: true }]}
              empty={<EmptyState title="Không có lệnh đang mở" />}
            />
          </section>
          <div className="grid gap-6 xl:grid-cols-3">
            <section className="xl:col-span-2">
              <div className="mb-2 flex items-center justify-between">
                <h2 className="font-medium">Hàng đợi cần chú ý</h2>
                <Button variant="link" size="sm" asChild>
                  <Link to="/ema1d/todos">Toàn bộ hàng đợi →</Link>
                </Button>
              </div>
              <TodoTable todos={status.todos} />
            </section>
            <LastSyncCard status={status} />
          </div>
        </>
      )}
    </>
  )
}
