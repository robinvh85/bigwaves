import { useQuery } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { z } from 'zod'

import { todosQuery } from '@/features/ema1d/api'
import { ManualTodoDialog } from '@/features/ema1d/components/ManualTodoDialog'
import { TodoTable } from '@/features/ema1d/components/TodoTable'
import { PageHeader } from '@/shared/components/states'
import { ToggleGroup, ToggleGroupItem } from '@/shared/ui/toggle-group'

export const Route = createFileRoute('/_app/ema1d/todos')({
  staticData: { title: 'Queue' },
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
        title="Order queue"
        description="OPEN/CLOSE todos from the sync or created by hand. The queue runs every minute: CLOSE before OPEN, transient errors retried after 1, 2, 5, 10, 30, 60 minutes."
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
        <ToggleGroupItem value="attention">Pending & needs attention</ToggleGroupItem>
        <ToggleGroupItem value="all">All, last 30 days</ToggleGroupItem>
      </ToggleGroup>
      <TodoTable todos={data} isLoading={isLoading} error={error} onRetry={() => void refetch()} />
    </>
  )
}
