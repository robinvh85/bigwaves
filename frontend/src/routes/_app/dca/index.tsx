import { useQuery } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { createColumnHelper } from '@tanstack/react-table'

import { type DcaSetting, dcaListQuery } from '@/features/dca/api'
import { SettingStatusBadge } from '@/features/dca/components/badges'
import { CreateLadderDialog } from '@/features/dca/components/CreateLadderDialog'
import { DataTable } from '@/shared/components/data-table'
import { Money, Pnl, Price } from '@/shared/components/numbers'
import { EmptyState, PageHeader } from '@/shared/components/states'
import { StatCard } from '@/shared/components/stat-card'

export const Route = createFileRoute('/_app/dca/')({
  staticData: { title: 'Ladders' },
  component: DcaList,
})

const col = createColumnHelper<DcaSetting>()
const num = (v?: string) => (v ? Number(v) : -Infinity)
const columns = [
  col.accessor('pairName', {
    header: 'Pair',
    cell: (c) => <span className="font-mono font-medium">{c.getValue()}</span>,
    meta: { mobile: 'title' },
  }),
  col.accessor('status', {
    header: 'Status',
    cell: (c) => <SettingStatusBadge status={c.getValue()} />,
    meta: { mobile: 'badge' },
  }),
  col.accessor('account', {
    header: 'Account',
    cell: (c) => <span className="text-muted-foreground">{c.getValue()}</span>,
    meta: { mobile: 'hidden' },
  }),
  col.accessor((s) => num(s.currentPrice), {
    id: 'price',
    header: 'Price',
    cell: (c) => <Price value={c.row.original.currentPrice} updatedAt={c.row.original.priceUpdatedAt} />,
    meta: { align: 'right' },
  }),
  col.accessor((s) => num(s.nextBuyPrice), {
    id: 'nextBuy',
    header: 'Next buy',
    cell: (c) => <Price value={c.row.original.nextBuyPrice} className="text-primary" />,
    meta: { align: 'right' },
  }),
  col.accessor((s) => num(s.nextSellPrice), {
    id: 'nextSell',
    header: 'Next sell',
    cell: (c) => <Price value={c.row.original.nextSellPrice} className="text-long" />,
    meta: { align: 'right' },
  }),
  col.display({
    id: 'counts',
    header: 'Rungs (waiting / held)',
    cell: (c) => (
      <span className="num">
        {c.row.original.waitingCount} / {c.row.original.holdingCount}
      </span>
    ),
  }),
  col.accessor((s) => num(s.capitalUsdt), {
    id: 'capital',
    header: 'Capital held',
    cell: (c) => <Money value={c.row.original.capitalUsdt} />,
    meta: { align: 'right' },
  }),
  col.accessor((s) => num(s.unrealizedProfit) + num(s.realizedProfit), {
    id: 'pnl',
    header: 'PnL (realized + unrealized)',
    cell: (c) => (
      <div className="flex flex-col items-end">
        <Pnl value={c.row.original.realizedProfit} />
        <Pnl value={c.row.original.unrealizedProfit} className="text-xs" />
      </div>
    ),
    meta: { align: 'right', mobile: 'primary' },
  }),
]

function sum(rows: DcaSetting[] | undefined, key: 'capitalUsdt' | 'realizedProfit' | 'unrealizedProfit') {
  return rows ? String(rows.reduce((a, r) => a + Number(r[key]), 0)) : undefined
}

function DcaList() {
  const navigate = useNavigate()
  const { data, isLoading, error, refetch } = useQuery(dcaListQuery)
  return (
    <>
      <PageHeader
        title="DCA"
        description="Buy ladders you set up: the system only watches prices (every 5 minutes) and fills with market orders."
        actions={<CreateLadderDialog />}
      />
      <div className="mb-4 grid grid-cols-2 gap-3 lg:grid-cols-3">
        <StatCard
          label="Capital held"
          loading={isLoading}
          value={<Money value={sum(data, 'capitalUsdt')} />}
        />
        <StatCard label="Realized" loading={isLoading} value={<Pnl value={sum(data, 'realizedProfit')} />} />
        <StatCard
          label="Unrealized"
          loading={isLoading}
          value={<Pnl value={sum(data, 'unrealizedProfit')} />}
        />
      </div>
      <DataTable
        columns={columns}
        data={data}
        isLoading={isLoading}
        error={error}
        onRetry={() => void refetch()}
        getRowId={(s) => String(s.id)}
        onRowClick={(s) => void navigate({ to: '/dca/$id', params: { id: String(s.id) } })}
        empty={
          <EmptyState
            title="No ladders yet"
            description="Create the first ladder, then add buy/sell rungs."
          />
        }
      />
    </>
  )
}
