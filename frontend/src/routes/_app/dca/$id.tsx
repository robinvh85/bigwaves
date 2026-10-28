import { useQuery } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { createColumnHelper } from '@tanstack/react-table'
import { PauseIcon, PencilIcon, PlayIcon, PlusIcon, Trash2Icon } from 'lucide-react'
import { useMemo, useState } from 'react'
import { z } from 'zod'

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
import { Money, Pct, Pnl, Price, Qty } from '@/shared/components/numbers'
import { EmptyState, ErrorState, PageHeader, SkeletonRows } from '@/shared/components/states'
import { StatCard } from '@/shared/components/stat-card'
import { formatDateTime } from '@/shared/lib/date'
import { percentOf } from '@/shared/lib/format'
import { Button } from '@/shared/ui/button'
import { Checkbox } from '@/shared/ui/checkbox'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/shared/ui/dropdown-menu'

export const Route = createFileRoute('/_app/dca/$id')({
  staticData: { title: 'Ladder', parent: { title: 'Ladders', to: '/dca' } },
  // all: also list sold rungs (finished trades); by default only the ones waiting to buy or holding.
  validateSearch: z.object({ all: z.boolean().optional() }),
  component: DcaDetail,
})

type Pending = { action: 'buy' | 'sell' | 'delete'; rung: DcaTrade }
const col = createColumnHelper<DcaTrade>()

function DcaDetail() {
  const id = Number(Route.useParams().id)
  const { all = false } = Route.useSearch()
  const navigate = useNavigate({ from: Route.fullPath })
  const { data: s, error, refetch } = useQuery(dcaDetailQuery(id))
  const update = useUpdateSetting(id)
  const del = useDeleteSetting()
  const action = useTradeAction()
  const [edit, setEdit] = useState<{ rung?: DcaTrade } | null>(null)
  const [pending, setPending] = useState<Pending | null>(null)
  const [confirmDelete, setConfirmDelete] = useState(false)
  // Memoized: a new array on every render makes DataTable rebuild its rows and reset its page,
  // which renders again.
  const trades = s?.trades
  const unsold = useMemo(() => (trades ?? []).filter((t) => t.status !== 'CLOSED'), [trades])

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
      header: 'Status',
      cell: (c) => <TradeStatusBadge status={c.getValue()} />,
      meta: { mobile: 'badge' },
    }),
    col.accessor((t) => Number(t.buyPrice), {
      id: 'buy',
      header: 'Buy ≤',
      cell: (c) => <Price value={c.row.original.buyPrice} />,
      meta: { align: 'right' },
    }),
    col.accessor((t) => Number(t.sellPrice), {
      id: 'sell',
      header: 'Sell ≥',
      cell: (c) =>
        c.row.original.sellPrice === '0' ? (
          <span className="text-muted-foreground">hold</span>
        ) : (
          <Price value={c.row.original.sellPrice} />
        ),
      meta: { align: 'right' },
    }),
    col.accessor('amount', {
      header: `Quantity (${coin})`,
      cell: (c) => (
        <Qty value={c.row.original.status === 'OPENING' ? c.getValue() : c.row.original.heldAmount} />
      ),
      meta: { align: 'right' },
    }),
    col.accessor('openPrice', {
      header: 'Buy fill',
      cell: (c) => <Price value={c.getValue()} />,
      meta: { align: 'right', mobile: 'hidden' },
    }),
    col.accessor('closePrice', {
      header: 'Sell fill',
      cell: (c) => <Price value={c.getValue()} />,
      meta: { align: 'right', mobile: 'hidden' },
    }),
    col.accessor((t) => Number(t.realizedProfit) + Number(t.unrealizedProfit), {
      id: 'pnl',
      header: 'PnL',
      cell: (c) => {
        const t = c.row.original
        if (t.status === 'OPENING') return <span className="text-muted-foreground">—</span>
        const pnl = t.status === 'CLOSED' ? t.realizedProfit : t.unrealizedProfit
        const pct = percentOf(pnl, t.costUsdt)
        return (
          <div className="flex flex-col items-end">
            <Pnl value={pnl} />
            {pct !== null && <Pct value={pct} className="text-xs" />}
          </div>
        )
      },
      meta: { align: 'right', mobile: 'primary' },
    }),
    col.accessor('lastError', {
      header: 'Error',
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
      meta: { align: 'right', label: 'Actions' },
      cell: (c) => {
        const t = c.row.original
        if (t.status === 'CLOSED')
          return <span className="text-xs text-muted-foreground">{formatDateTime(t.closeAt)}</span>
        return (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button size="sm" variant="ghost">
                Actions
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onSelect={() => setEdit({ rung: t })}>
                <PencilIcon />
                {t.status === 'OPEN' ? 'Edit sell price' : 'Edit rung'}
              </DropdownMenuItem>
              {t.status === 'OPENING' ? (
                <>
                  <DropdownMenuItem onSelect={() => setPending({ action: 'buy', rung: t })}>
                    Buy now (market)
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    className="text-destructive"
                    onSelect={() => setPending({ action: 'delete', rung: t })}
                  >
                    <Trash2Icon />
                    Delete rung
                  </DropdownMenuItem>
                </>
              ) : (
                <DropdownMenuItem onSelect={() => setPending({ action: 'sell', rung: t })}>
                  Sell now (market)
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
            <SettingStatusBadge status={s.status} /> {s.account} · current price{' '}
            <Price value={s.currentPrice} updatedAt={s.priceUpdatedAt} />
            {s.note && <span>· {s.note}</span>}
          </span>
        }
        actions={
          <>
            {s.status === 'ACTIVE' ? (
              <Button variant="outline" onClick={() => update.mutate({ status: 'INACTIVE' })}>
                <PauseIcon />
                Pause
              </Button>
            ) : (
              <Button variant="outline" onClick={() => update.mutate({ status: 'ACTIVE' })}>
                <PlayIcon />
                Resume
              </Button>
            )}
            <Button variant="outline" className="text-destructive" onClick={() => setConfirmDelete(true)}>
              <Trash2Icon />
              Delete
            </Button>
            <Button onClick={() => setEdit({})}>
              <PlusIcon />
              Add rung
            </Button>
          </>
        }
      />
      <div className="mb-4 grid grid-cols-2 gap-3 lg:grid-cols-4">
        <StatCard
          label="Holding"
          value={<Qty value={s.heldAmount} />}
          hint={`${s.holdingCount} rungs · ${s.waitingCount} waiting to buy`}
        />
        <StatCard label="Capital held" value={<Money value={s.capitalUsdt} />} />
        <StatCard label="Realized" value={<Pnl value={s.realizedProfit} />} />
        <StatCard label="Unrealized" value={<Pnl value={s.unrealizedProfit} />} />
      </div>
      <div className="grid gap-6 2xl:grid-cols-3">
        <div className="space-y-2 2xl:col-span-2">
          <label className="flex w-fit items-center gap-2 text-sm">
            <Checkbox
              checked={all}
              onCheckedChange={(v) =>
                void navigate({
                  search: (prev) => ({ ...prev, all: v === true || undefined }),
                  replace: true,
                })
              }
            />
            Show sold rungs
            <span className="text-muted-foreground">({(trades?.length ?? 0) - unsold.length} sold)</span>
          </label>
          <DataTable
            columns={columns}
            data={all ? trades : unsold}
            getRowId={(t) => String(t.id)}
            sorting={[{ id: 'buy', desc: true }]}
            empty={
              all || !trades?.length ? (
                <EmptyState
                  title="No rungs yet"
                  description="Add the first rung: the buy price, how many coins and the sell price."
                />
              ) : (
                <EmptyState title="All rungs are sold" description="Tick “Show sold rungs” to see them." />
              )
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
              ? `Buy ${pending.rung.amount} ${coin} now?`
              : pending.action === 'sell'
                ? `Sell ${pending.rung.heldAmount} ${coin} now?`
                : `Delete rung #${pending.rung.id}?`
          }
          description={
            pending.action === 'delete'
              ? 'The rung has not bought yet and will be deleted.'
              : `A market order goes straight to Binance at the current price (~${s.currentPrice ?? '?'}), without waiting for the trigger price.`
          }
          confirmLabel={pending.action === 'buy' ? 'Buy' : pending.action === 'sell' ? 'Sell' : 'Delete'}
          onConfirm={() => action.mutateAsync({ id: pending.rung.id, action: pending.action })}
        />
      )}
      <ConfirmDialog
        open={confirmDelete}
        onOpenChange={setConfirmDelete}
        destructive
        title={`Delete ladder ${s.pairName}?`}
        description="All rungs (history included) are deleted. Not possible while a rung still holds coins."
        typeToConfirm={s.pairName}
        confirmLabel="Delete ladder"
        onConfirm={async () => {
          await del.mutateAsync(id)
          void navigate({ to: '/dca' })
        }}
      />
    </>
  )
}
