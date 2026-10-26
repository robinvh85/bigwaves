import { useQuery } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { createColumnHelper } from '@tanstack/react-table'
import { PauseIcon, PencilIcon, PlayIcon, PlusIcon, Trash2Icon } from 'lucide-react'
import { useState } from 'react'

import {
  type DcaTrade,
  dcaDetailQuery,
  useDeleteSetting,
  useTradeAction,
  useUpdateSetting,
} from '@/features/dca/api'
import { SettingStatusBadge, TradeStatusBadge } from '@/features/dca/components/badges'
import { PriceLadder } from '@/features/dca/components/PriceLadder'
import { RungDialog } from '@/features/dca/components/RungDialog'
import { ConfirmDialog } from '@/shared/components/confirm-dialog'
import { DataTable } from '@/shared/components/data-table'
import { Money, Pnl, Price, Qty } from '@/shared/components/numbers'
import { EmptyState, ErrorState, PageHeader, SkeletonRows } from '@/shared/components/states'
import { StatCard } from '@/shared/components/stat-card'
import { formatDateTime } from '@/shared/lib/date'
import { Button } from '@/shared/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/shared/ui/dropdown-menu'

export const Route = createFileRoute('/_app/dca/$id')({
  staticData: { title: 'Ladder' },
  component: DcaDetail,
})

type Pending = { action: 'buy' | 'sell' | 'delete'; rung: DcaTrade }
const col = createColumnHelper<DcaTrade>()

function DcaDetail() {
  const id = Number(Route.useParams().id)
  const navigate = useNavigate()
  const { data: s, error, refetch } = useQuery(dcaDetailQuery(id))
  const update = useUpdateSetting(id)
  const del = useDeleteSetting()
  const action = useTradeAction()
  const [edit, setEdit] = useState<{ rung?: DcaTrade } | null>(null)
  const [pending, setPending] = useState<Pending | null>(null)
  const [confirmDelete, setConfirmDelete] = useState(false)

  if (error) return <ErrorState error={error} onRetry={() => void refetch()} />
  if (!s) return <SkeletonRows rows={8} />
  const coin = s.pairName.replace(/USDT$/, '')

  const columns = [
    col.accessor('id', {
      header: '#',
      cell: (c) => <span className="num text-muted-foreground">{c.getValue()}</span>,
      meta: { mobile: 'title' },
    }),
    col.accessor('status', {
      header: 'Trạng thái',
      cell: (c) => <TradeStatusBadge status={c.getValue()} />,
      meta: { mobile: 'badge' },
    }),
    col.accessor((t) => Number(t.buyPrice), {
      id: 'buy',
      header: 'Mua ≤',
      cell: (c) => <Price value={c.row.original.buyPrice} />,
      meta: { align: 'right' },
    }),
    col.accessor((t) => Number(t.sellPrice), {
      id: 'sell',
      header: 'Bán ≥',
      cell: (c) =>
        c.row.original.sellPrice === '0' ? (
          <span className="text-muted-foreground">giữ</span>
        ) : (
          <Price value={c.row.original.sellPrice} />
        ),
      meta: { align: 'right' },
    }),
    col.accessor('amount', {
      header: `Khối lượng (${coin})`,
      cell: (c) => (
        <Qty value={c.row.original.status === 'OPENING' ? c.getValue() : c.row.original.heldAmount} />
      ),
      meta: { align: 'right' },
    }),
    col.accessor('openPrice', {
      header: 'Giá khớp mua',
      cell: (c) => <Price value={c.getValue()} />,
      meta: { align: 'right', mobile: 'hidden' },
    }),
    col.accessor('closePrice', {
      header: 'Giá khớp bán',
      cell: (c) => <Price value={c.getValue()} />,
      meta: { align: 'right', mobile: 'hidden' },
    }),
    col.accessor((t) => Number(t.realizedProfit) + Number(t.unrealizedProfit), {
      id: 'pnl',
      header: 'Lãi/lỗ',
      cell: (c) => {
        const t = c.row.original
        if (t.status === 'OPENING') return <span className="text-muted-foreground">—</span>
        return <Pnl value={t.status === 'CLOSED' ? t.realizedProfit : t.unrealizedProfit} />
      },
      meta: { align: 'right', mobile: 'primary' },
    }),
    col.accessor('lastError', {
      header: 'Lỗi',
      enableSorting: false,
      cell: (c) =>
        c.getValue() ? (
          <span className="line-clamp-2 max-w-60 text-xs text-destructive">{c.getValue()}</span>
        ) : null,
      meta: { mobile: 'hidden' },
    }),
    col.display({
      id: 'actions',
      header: '',
      meta: { align: 'right', label: 'Thao tác' },
      cell: (c) => {
        const t = c.row.original
        if (t.status === 'CLOSED')
          return <span className="text-xs text-muted-foreground">{formatDateTime(t.closeAt)}</span>
        return (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button size="sm" variant="ghost">
                Thao tác
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onSelect={() => setEdit({ rung: t })}>
                <PencilIcon />
                {t.status === 'OPEN' ? 'Sửa giá bán' : 'Sửa nấc'}
              </DropdownMenuItem>
              {t.status === 'OPENING' ? (
                <>
                  <DropdownMenuItem onSelect={() => setPending({ action: 'buy', rung: t })}>
                    Mua ngay (market)
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    className="text-destructive"
                    onSelect={() => setPending({ action: 'delete', rung: t })}
                  >
                    <Trash2Icon />
                    Xoá nấc
                  </DropdownMenuItem>
                </>
              ) : (
                <DropdownMenuItem onSelect={() => setPending({ action: 'sell', rung: t })}>
                  Bán ngay (market)
                </DropdownMenuItem>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        )
      },
    }),
  ]

  return (
    <>
      <PageHeader
        title={s.pairName}
        description={
          <span className="inline-flex flex-wrap items-center gap-2">
            <SettingStatusBadge status={s.status} /> {s.account} · giá hiện tại{' '}
            <Price value={s.currentPrice} />
            {s.note && <span>· {s.note}</span>}
          </span>
        }
        actions={
          <>
            {s.status === 'ACTIVE' ? (
              <Button variant="outline" onClick={() => update.mutate({ status: 'INACTIVE' })}>
                <PauseIcon />
                Tạm dừng
              </Button>
            ) : (
              <Button variant="outline" onClick={() => update.mutate({ status: 'ACTIVE' })}>
                <PlayIcon />
                Chạy
              </Button>
            )}
            <Button variant="outline" className="text-destructive" onClick={() => setConfirmDelete(true)}>
              <Trash2Icon />
              Xoá
            </Button>
            <Button onClick={() => setEdit({})}>
              <PlusIcon />
              Thêm nấc
            </Button>
          </>
        }
      />
      <div className="mb-4 grid grid-cols-2 gap-3 lg:grid-cols-4">
        <StatCard
          label="Đang giữ"
          value={<Qty value={s.heldAmount} />}
          hint={`${s.holdingCount} nấc · ${s.waitingCount} nấc chờ mua`}
        />
        <StatCard label="Vốn đang giữ" value={<Money value={s.capitalUsdt} />} />
        <StatCard label="Đã chốt" value={<Pnl value={s.realizedProfit} />} />
        <StatCard label="Chưa chốt" value={<Pnl value={s.unrealizedProfit} />} />
      </div>
      <div className="grid gap-6 2xl:grid-cols-3">
        <div className="2xl:col-span-2">
          <DataTable
            columns={columns}
            data={s.trades}
            getRowId={(t) => String(t.id)}
            sorting={[{ id: 'buy', desc: true }]}
            empty={
              <EmptyState
                title="Chưa có nấc nào"
                description="Thêm nấc đầu tiên: mua ở giá nào, bao nhiêu coin, bán ở giá nào."
              />
            }
          />
        </div>
        <PriceLadder setting={s} />
      </div>

      {edit && <RungDialog setting={s} rung={edit.rung} open onOpenChange={(o) => !o && setEdit(null)} />}
      {pending && (
        <ConfirmDialog
          open
          onOpenChange={(o) => !o && setPending(null)}
          destructive={pending.action !== 'buy'}
          title={
            pending.action === 'buy'
              ? `Mua ngay ${pending.rung.amount} ${coin}?`
              : pending.action === 'sell'
                ? `Bán ngay ${pending.rung.heldAmount} ${coin}?`
                : `Xoá nấc #${pending.rung.id}?`
          }
          description={
            pending.action === 'delete'
              ? 'Nấc chưa mua sẽ bị xoá.'
              : `Lệnh market gửi thẳng lên Binance ở giá hiện tại (~${s.currentPrice ?? '?'}), không chờ giá kích hoạt.`
          }
          confirmLabel={pending.action === 'buy' ? 'Mua' : pending.action === 'sell' ? 'Bán' : 'Xoá'}
          onConfirm={() => action.mutateAsync({ id: pending.rung.id, action: pending.action })}
        />
      )}
      <ConfirmDialog
        open={confirmDelete}
        onOpenChange={setConfirmDelete}
        destructive
        title={`Xoá ladder ${s.pairName}?`}
        description="Mọi nấc (kể cả lịch sử) bị xoá. Không xoá được khi còn nấc đang giữ coin."
        typeToConfirm={s.pairName}
        confirmLabel="Xoá ladder"
        onConfirm={async () => {
          await del.mutateAsync(id)
          void navigate({ to: '/dca' })
        }}
      />
    </>
  )
}
