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
          Huỷ
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
      header: 'Lệnh',
      cell: (c) => (
        <span className="inline-flex items-center gap-1.5">
          <span className="text-xs font-semibold">{c.row.original.action}</span>
          <Side side={c.row.original.side} />
        </span>
      ),
      meta: { mobile: 'badge' },
    }),
    col.accessor('status', {
      header: 'Trạng thái',
      cell: (c) => <TodoStatusBadge status={c.getValue()} />,
      meta: { mobile: 'primary' },
    }),
    col.accessor('signalDate', { header: 'Tín hiệu', cell: (c) => formatDate(c.getValue()) }),
    col.accessor('refPrice', {
      header: 'Giá tham chiếu',
      cell: (c) => <Price value={c.getValue()} />,
      meta: { align: 'right' },
    }),
    col.accessor('attempts', {
      header: 'Lần thử',
      cell: (c) => {
        const t = c.row.original
        return (
          <span className="num">
            {t.attempts}
            {t.runAfter && t.status === 'PENDING' && (
              <span className="ml-1 text-xs text-muted-foreground">· thử lại {fromNow(t.runAfter)}</span>
            )}
          </span>
        )
      },
    }),
    col.accessor('lastError', {
      header: 'Lỗi gần nhất',
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
      header: 'Cập nhật',
      cell: (c) => <span title={formatDateTime(c.getValue())}>{fromNow(c.getValue())}</span>,
      meta: { align: 'right', mobile: 'hidden' },
    }),
    col.display({
      id: 'actions',
      header: '',
      cell: (c) => (
        <Actions todo={c.row.original} onAction={(action, todo) => setPending({ action, todo })} />
      ),
      meta: { align: 'right', label: 'Thao tác' },
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
        empty={<EmptyState title="Hàng đợi trống" description="Không có lệnh nào đang chờ hoặc cần xử lý." />}
      />
      {pending && (
        <ConfirmDialog
          open
          onOpenChange={(o) => {
            if (!o) setPending(null)
          }}
          title={
            pending.action === 'retry'
              ? `Chạy lại todo #${pending.todo.id}?`
              : `Huỷ todo #${pending.todo.id}?`
          }
          description={
            pending.action === 'retry'
              ? `${pending.todo.action} ${pending.todo.side} ${pending.todo.coin} sẽ quay lại hàng đợi với 10 lần thử mới.`
              : `${pending.todo.action} ${pending.todo.side} ${pending.todo.coin} chưa từng gửi lên Binance và sẽ bị huỷ.`
          }
          destructive={pending.action === 'cancel'}
          confirmLabel={pending.action === 'retry' ? 'Chạy lại' : 'Huỷ todo'}
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
