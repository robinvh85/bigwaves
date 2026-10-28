import { useQuery } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { createColumnHelper } from '@tanstack/react-table'
import { AlertTriangleIcon, Loader2Icon, SearchCheckIcon } from 'lucide-react'
import { toast } from 'sonner'
import { z } from 'zod'

import { type Ema1dPairRow, pairsQuery, useCheckPairs } from '@/features/ema1d/api'
import { BasketPairSheet } from '@/features/ema1d/components/BasketPairSheet'
import { BasketStatusBadge, GroupBadge, WantedLabel } from '@/features/ema1d/components/badges'
import { DataTable } from '@/shared/components/data-table'
import { Pct, Price, Side } from '@/shared/components/numbers'
import { PageHeader } from '@/shared/components/states'
import { formatDate } from '@/shared/lib/date'
import { Button } from '@/shared/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/shared/ui/tooltip'

export const Route = createFileRoute('/_app/ema1d/pairs')({
  staticData: { title: 'Basket' },
  validateSearch: z.object({ pair: z.number().int().optional() }),
  component: BasketPage,
})

const col = createColumnHelper<Ema1dPairRow>()
const columns = [
  col.accessor('coin', {
    header: 'Coin',
    cell: (c) => (
      <span className="inline-flex items-center gap-1.5 font-mono font-medium">
        {c.getValue()}
        {c.row.original.mismatch && (
          <Tooltip>
            <TooltipTrigger asChild>
              <AlertTriangleIcon
                className="size-3.5 text-warning"
                aria-label="Out of line with the strategy"
              />
            </TooltipTrigger>
            <TooltipContent>Position differs from the strategy and no todo is fixing it</TooltipContent>
          </Tooltip>
        )}
      </span>
    ),
    meta: { mobile: 'title' },
  }),
  col.accessor('status', {
    header: 'Status',
    cell: (c) => <BasketStatusBadge status={c.getValue()} />,
    meta: { mobile: 'badge' },
  }),
  col.accessor('pairGroup', { header: 'Group', cell: (c) => <GroupBadge group={c.getValue()} /> }),
  col.display({
    id: 'sides',
    header: 'Allowed sides',
    cell: (c) => {
      const p = c.row.original
      return (
        <span className="text-sm">
          {p.allowLong ? 'L' : '–'}
          {' / '}
          {p.allowShort ? p.shortSupported ? 'S' : <span className="text-warning">S?</span> : '–'}
          <span className="ml-1 text-xs text-muted-foreground">{p.longExitRule}</span>
        </span>
      )
    },
    meta: { mobile: 'hidden' },
  }),
  col.display({
    id: 'wanted',
    header: 'Strategy',
    cell: (c) => (c.row.original.wanted ? <WantedLabel state={c.row.original.wanted.state} /> : '—'),
  }),
  col.accessor((p) => p.position?.side ?? '', {
    id: 'holding',
    header: 'Holding',
    cell: (c) => {
      const pos = c.row.original.position
      return pos ? (
        <span className="inline-flex items-center gap-1.5">
          <Side side={pos.side} />
          <span className="text-xs text-muted-foreground">{formatDate(pos.signalDate)}</span>
        </span>
      ) : (
        <span className="text-muted-foreground">—</span>
      )
    },
  }),
  col.accessor((p) => (p.position?.unrealizedPercent ? Number(p.position.unrealizedPercent) : -Infinity), {
    id: 'pnl',
    header: 'PnL',
    cell: (c) => <Pct value={c.row.original.position?.unrealizedPercent} />,
    meta: { align: 'right', mobile: 'primary' },
  }),
  col.accessor((p) => p.pendingTodos.join(', '), {
    id: 'pending',
    header: 'Pending',
    cell: (c) => <span className="text-xs">{c.getValue() || '—'}</span>,
    meta: { mobile: 'hidden' },
  }),
  col.accessor((p) => (p.currentPrice ? Number(p.currentPrice) : 0), {
    id: 'price',
    header: 'Price',
    cell: (c) => <Price value={c.row.original.currentPrice} updatedAt={c.row.original.priceUpdatedAt} />,
    meta: { align: 'right' },
  }),
]

function BasketPage() {
  const { pair } = Route.useSearch()
  const navigate = useNavigate({ from: Route.fullPath })
  const { data, isLoading, error, refetch } = useQuery(pairsQuery)
  const check = useCheckPairs()

  return (
    <>
      <PageHeader
        title="Basket"
        description="Each coin: the held position next to what the latest sync wants. ⚠ marks a difference no todo is fixing."
        actions={
          <Button
            variant="outline"
            disabled={check.isPending}
            onClick={() =>
              check.mutate(undefined, {
                onSuccess: (r) => toast.success(`Checked ${r.items.length} pairs on Binance`),
              })
            }
          >
            {check.isPending ? <Loader2Icon className="animate-spin" /> : <SearchCheckIcon />}
            Check whole basket
          </Button>
        }
      />
      <DataTable
        columns={columns}
        data={data}
        isLoading={isLoading}
        error={error}
        onRetry={() => void refetch()}
        getRowId={(p) => String(p.id)}
        onRowClick={(p) => void navigate({ search: { pair: p.id }, replace: true })}
        rowClassName={(p) => (p.status === 'DISABLED' ? 'opacity-60' : undefined)}
      />
      <BasketPairSheet id={pair} onClose={() => void navigate({ search: {}, replace: true })} />
    </>
  )
}
