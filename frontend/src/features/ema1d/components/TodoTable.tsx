import { createColumnHelper } from '@tanstack/react-table'
import { RotateCcwIcon, XIcon } from 'lucide-react'
import { useState } from 'react'

import { ConfirmDialog } from '@/shared/components/confirm-dialog'
import { DataTable } from '@/shared/components/data-table'
import { Price, Side } from '@/shared/components/numbers'
import { EmptyState } from '@/shared/components/states'
import { formatDate, formatDateTime, fromNow } from '@/shared/lib/date'
import { Button } from '@/shared/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/shared/ui/tooltip'

import { type Ema1dTodo, useCancelTodo, useRetryTodo } from '../api'
import { TodoStatusBadge } from './badges'

const col = createColumnHelper<Ema1dTodo>()

function Actions({
  todo,
  onAction,
}: {
  todo: Ema1dTodo
  onAction: (a: 'retry' | 'cancel', t: Ema1dTodo) => void
}) {
  return (
    <div className="flex justify-end gap-1">
      {todo.retryable && (
        <Button size="sm" variant="outline" onClick={() => onAction('retry', todo)}>
          <RotateCcwIcon />
          Retry
        </Button>
      )}
      {todo.cancellable && (
        <Button
          size="sm"
          variant="ghost"
          className="text-destructive"
          onClick={() => onAction('cancel', todo)}
        >
          <XIcon />
          Cancel
        </Button>
      )}
    </div>
  )
}

export function TodoTable({
  todos,
  isLoading,
  error,
  onRetry,
}: {
  todos?: Ema1dTodo[]
  isLoading?: boolean
  error?: unknown
  onRetry?: () => void
}) {
  const retry = useRetryTodo()
  const cancel = useCancelTodo()
  const [pending, setPending] = useState<{ action: 'retry' | 'cancel'; todo: Ema1dTodo } | null>(null)

  const columns = [
    col.accessor('id', {
      header: '#',
      cell: (c) => <span className="num text-muted-foreground">{c.getValue()}</span>,
      meta: { mobile: 'hidden' },
    }),
    col.accessor('coin', {
      header: 'Coin',
      cell: (c) => <span className="font-mono font-medium">{c.getValue()}</span>,
      meta: { mobile: 'title' },
    }),
    col.display({
      id: 'what',
      header: 'Order',
      cell: (c) => (
        <span className="inline-flex items-center gap-1.5">
          <span className="text-xs font-semibold">{c.row.original.action}</span>
          <Side side={c.row.original.side} />
        </span>
      ),
      meta: { mobile: 'badge' },
    }),
    col.accessor('status', {
      header: 'Status',
      cell: (c) => <TodoStatusBadge status={c.getValue()} />,
      meta: { mobile: 'primary' },
    }),
    col.accessor('signalDate', { header: 'Signal', cell: (c) => formatDate(c.getValue()) }),
    col.accessor('refPrice', {
      header: 'Ref. price',
      cell: (c) => <Price value={c.getValue()} />,
      meta: { align: 'right' },
    }),
    col.accessor('attempts', {
      header: 'Attempts',
      cell: (c) => {
        const t = c.row.original
        return (
          <span className="num">
            {t.attempts}
            {t.runAfter && t.status === 'PENDING' && (
              <span className="ml-1 text-xs text-muted-foreground">· retry {fromNow(t.runAfter)}</span>
            )}
          </span>
        )
      },
    }),
    col.accessor('lastError', {
      header: 'Last error',
      enableSorting: false,
      cell: (c) =>
        c.getValue() ? (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="line-clamp-2 max-w-80 text-xs text-destructive">{c.getValue()}</span>
            </TooltipTrigger>
            <TooltipContent className="max-w-lg break-words">{c.getValue()}</TooltipContent>
          </Tooltip>
        ) : (
          <span className="text-muted-foreground">—</span>
        ),
      meta: { className: 'max-w-80' },
    }),
    col.accessor('updatedAt', {
      header: 'Updated',
      cell: (c) => <span title={formatDateTime(c.getValue())}>{fromNow(c.getValue())}</span>,
      meta: { align: 'right', mobile: 'hidden' },
    }),
    col.display({
      id: 'actions',
      header: '',
      cell: (c) => (
        <Actions todo={c.row.original} onAction={(action, todo) => setPending({ action, todo })} />
      ),
      meta: { align: 'right', label: 'Actions' },
    }),
  ]

  return (
    <>
      <DataTable
        columns={columns}
        data={todos}
        isLoading={isLoading}
        error={error}
        onRetry={onRetry}
        getRowId={(t) => String(t.id)}
        sorting={[{ id: 'id', desc: true }]}
        rowClassName={(t) => (t.status === 'FAILED' ? 'bg-destructive/5' : undefined)}
        empty={<EmptyState title="Queue is empty" description="Nothing is waiting or needs attention." />}
      />
      {pending && (
        <ConfirmDialog
          open
          onOpenChange={(o) => {
            if (!o) setPending(null)
          }}
          title={
            pending.action === 'retry'
              ? `Retry todo #${pending.todo.id}?`
              : `Cancel todo #${pending.todo.id}?`
          }
          description={
            pending.action === 'retry'
              ? `${pending.todo.action} ${pending.todo.side} ${pending.todo.coin} goes back to the queue with 10 fresh attempts.`
              : `${pending.todo.action} ${pending.todo.side} ${pending.todo.coin} was never sent to Binance and will be cancelled.`
          }
          destructive={pending.action === 'cancel'}
          confirmLabel={pending.action === 'retry' ? 'Retry' : 'Cancel todo'}
          onConfirm={() =>
            pending.action === 'retry'
              ? retry.mutateAsync(pending.todo.id)
              : cancel.mutateAsync(pending.todo.id)
          }
        />
      )}
    </>
  )
}
