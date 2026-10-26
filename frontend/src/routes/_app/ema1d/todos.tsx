import { useQuery } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { z } from 'zod'

import { todosQuery } from '@/features/ema1d/api'
import { ManualTodoDialog } from '@/features/ema1d/components/ManualTodoDialog'
import { TodoTable } from '@/features/ema1d/components/TodoTable'
import { PageHeader } from '@/shared/components/states'
import { ToggleGroup, ToggleGroupItem } from '@/shared/ui/toggle-group'

export const Route = createFileRoute('/_app/ema1d/todos')({
  staticData: { title: 'Hàng đợi' },
  validateSearch: z.object({ history: z.boolean().optional() }),
  component: TodosPage,
})

function TodosPage() {
  const { history = false } = Route.useSearch()
  const navigate = useNavigate({ from: Route.fullPath })
  const { data, isLoading, error, refetch } = useQuery(todosQuery(history))

  return (
    <>
      <PageHeader
        title="Hàng đợi lệnh"
        description="Todo OPEN/CLOSE do sync hoặc tạo tay. Mỗi phút hàng đợi chạy một lượt: CLOSE trước OPEN, lỗi tạm thời được thử lại sau 1, 2, 5, 10, 30, 60 phút."
        actions={<ManualTodoDialog />}
      />
      <ToggleGroup
        type="single"
        variant="outline"
        size="sm"
        className="mb-3"
        value={history ? 'all' : 'attention'}
        onValueChange={(v) => {
          if (v) void navigate({ search: { history: v === 'all' ? true : undefined }, replace: true })
        }}
      >
        <ToggleGroupItem value="attention">Đang chờ & cần xử lý</ToggleGroupItem>
        <ToggleGroupItem value="all">Toàn bộ 30 ngày</ToggleGroupItem>
      </ToggleGroup>
      <TodoTable todos={data} isLoading={isLoading} error={error} onRetry={() => void refetch()} />
    </>
  )
}
