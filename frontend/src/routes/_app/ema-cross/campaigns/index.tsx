import { useQuery } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { createColumnHelper } from '@tanstack/react-table'
import { PlusIcon } from 'lucide-react'
import { useState } from 'react'
import { z } from 'zod'

import {
  type EmaSetting,
  emaSettingsConfigQuery,
  emaSettingsListQuery,
  TRADE_TYPE_LABEL,
} from '@/features/emasettings/api'
import { SettingStatusBadge } from '@/features/emasettings/components/badges'
import { SettingDialog } from '@/features/emasettings/components/SettingDialog'
import { DataTable } from '@/shared/components/data-table'
import { Money, Pct, Pnl, Price, Side } from '@/shared/components/numbers'
import { EmptyState, PageHeader } from '@/shared/components/states'
import { StatCard } from '@/shared/components/stat-card'
import { Alert, AlertDescription } from '@/shared/ui/alert'
import { Button } from '@/shared/ui/button'
import { ToggleGroup, ToggleGroupItem } from '@/shared/ui/toggle-group'

const STATUS_FILTERS = ['ACTIVE', 'INACTIVE', 'DONE', 'ALL'] as const
const FILTER_LABEL = { ACTIVE: 'Active', INACTIVE: 'Paused', DONE: 'Done', ALL: 'All' } as const

export const Route = createFileRoute('/_app/ema-cross/campaigns/')({
  staticData: { title: 'Campaigns' },
  validateSearch: z.object({ status: z.enum(STATUS_FILTERS).optional() }),
  component: CampaignList,
})

const col = createColumnHelper<EmaSetting>()
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
  col.display({
    id: 'rule',
    header: 'Rule',
    cell: (c) => {
      const s = c.row.original
      return (
        <span className="text-sm">
          {s.period} · {TRADE_TYPE_LABEL[s.tradeType]}
          <span className="block text-xs text-muted-foreground">
            {s.checkType.replace('CHECK_AT_', 'at ').toLowerCase()}
          </span>
        </span>
      )
    },
    meta: { mobile: 'hidden' },
  }),
  col.accessor((s) => num(s.budget), {
    id: 'budget',
    header: 'Budget',
    cell: (c) => <Money value={c.row.original.budget} />,
    meta: { align: 'right' },
  }),
  col.display({
    id: 'holding',
    header: 'Holding',
    cell: (c) => {
      const t = c.row.original.activeTrade
      if (!t) return <span className="text-muted-foreground">—</span>
      return (
        <span className="inline-flex items-center gap-1.5">
          <Side side={t.side} />
          {t.status === 'OPENING' ? (
            <span className="text-xs text-muted-foreground">waiting</span>
          ) : (
            <Price value={t.openPrice} />
          )}
        </span>
      )
    },
  }),
  col.accessor((s) => num(s.currentPrice), {
    id: 'price',
    header: 'Price',
    cell: (c) => <Price value={c.row.original.currentPrice} />,
    meta: { align: 'right', mobile: 'hidden' },
  }),
  col.accessor((s) => num(s.totalProfit), {
    id: 'pnl',
    header: 'Total PnL',
    cell: (c) => (
      <div className="flex flex-col items-end">
        <Pnl value={c.row.original.totalProfit} />
        <Pct value={c.row.original.totalPercent} className="text-xs" />
      </div>
    ),
    meta: { align: 'right', mobile: 'primary' },
  }),
  col.accessor('tradeCount', {
    header: 'Trades',
    cell: (c) => <span className="num">{c.getValue()}</span>,
    meta: { align: 'right', mobile: 'hidden' },
  }),
]

function total(rows: EmaSetting[] | undefined, pick: (s: EmaSetting) => string) {
  return rows ? String(rows.reduce((a, r) => a + Number(pick(r)), 0)) : undefined
}

function CampaignList() {
  const { status = 'ACTIVE' } = Route.useSearch()
  const navigate = useNavigate({ from: Route.fullPath })
  const { data, isLoading, error, refetch } = useQuery(
    emaSettingsListQuery(status === 'ALL' ? undefined : status),
  )
  const { data: cfg } = useQuery(emaSettingsConfigQuery)
  const [creating, setCreating] = useState(false)
  const held = data?.filter((s) => s.activeTrade?.status === 'OPEN')

  return (
    <>
      <PageHeader
        title="Campaigns"
        description="EMA14 / Smooth10 cross trading per pair with a budget, a stop loss per trade and a take profit / stop loss on the campaign (Rails: EMA Settings). Checked every 5 minutes."
        actions={
          <Button onClick={() => setCreating(true)}>
            <PlusIcon />
            New campaign
          </Button>
        }
      />
      {cfg && !cfg.config.enabled && (
        <Alert className="mb-4">
          <AlertDescription>
            The 5-minute scan is off (EMA Cross → Settings): campaigns open and close nothing on their own.
            Buttons on a campaign still place orders.
          </AlertDescription>
        </Alert>
      )}
      <div className="mb-4 grid grid-cols-2 gap-3 lg:grid-cols-3">
        <StatCard
          label="Held trades"
          loading={isLoading}
          value={<span className="num">{held?.length ?? 0}</span>}
        />
        <StatCard
          label="Unrealized"
          loading={isLoading}
          value={<Pnl value={total(data, (s) => s.unrealizedProfit)} />}
        />
        <StatCard
          label="Realized (listed campaigns)"
          loading={isLoading}
          value={<Pnl value={total(data, (s) => s.realizedProfit)} />}
        />
      </div>
      <ToggleGroup
        type="single"
        variant="outline"
        className="mb-3"
        value={status}
        onValueChange={(v) =>
          v && void navigate({ search: { status: v as (typeof STATUS_FILTERS)[number] } })
        }
      >
        {STATUS_FILTERS.map((s) => (
          <ToggleGroupItem key={s} value={s}>
            {FILTER_LABEL[s]}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
      <DataTable
        columns={columns}
        data={data}
        isLoading={isLoading}
        error={error}
        onRetry={() => void refetch()}
        getRowId={(s) => String(s.id)}
        onRowClick={(s) => void navigate({ to: '/ema-cross/campaigns/$id', params: { id: String(s.id) } })}
        rowClassName={(s) => (s.status === 'DONE' ? 'opacity-70' : undefined)}
        empty={
          <EmptyState title="No campaign" description="Create one: a pair, a budget and the cross rule." />
        }
      />
      <SettingDialog open={creating} onOpenChange={setCreating} />
    </>
  )
}
