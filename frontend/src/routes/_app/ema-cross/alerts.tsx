import { useQuery } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { createColumnHelper } from '@tanstack/react-table'
import { SearchIcon } from 'lucide-react'
import { z } from 'zod'

import { type EmaCrossAlert, emaCrossAlertsQuery, TIMEFRAMES } from '@/features/emacross/api'
import { Direction } from '@/features/emacross/components/badges'
import { DataTable } from '@/shared/components/data-table'
import { Pct, Price } from '@/shared/components/numbers'
import { EmptyState, PageHeader } from '@/shared/components/states'
import { formatDateTime } from '@/shared/lib/date'
import { Badge } from '@/shared/ui/badge'
import { Input } from '@/shared/ui/input'
import { ToggleGroup, ToggleGroupItem } from '@/shared/ui/toggle-group'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/shared/ui/tooltip'

const search = z.object({
  q: z.string().optional(),
  timeframe: z.enum(['H1', 'H4', 'D1']).optional(),
})

export const Route = createFileRoute('/_app/ema-cross/alerts')({
  staticData: { title: 'Alerts' },
  validateSearch: search,
  component: AlertsPage,
})

const col = createColumnHelper<EmaCrossAlert>()
const columns = [
  col.accessor('candleTime', {
    header: 'Candle',
    cell: (c) => <span className="whitespace-nowrap">{formatDateTime(c.getValue())}</span>,
    meta: { mobile: 'hidden' },
  }),
  col.accessor('pairName', {
    header: 'Pair',
    cell: (c) => <span className="font-mono font-medium">{c.getValue()}</span>,
    meta: { mobile: 'title' },
  }),
  col.accessor('timeframe', { header: 'Timeframe', meta: { mobile: 'badge' } }),
  col.accessor('direction', {
    header: 'Cross',
    cell: (c) => <Direction direction={c.getValue()} />,
  }),
  col.accessor((a) => Number(a.price), {
    id: 'price',
    header: 'Price',
    cell: (c) => <Price value={c.row.original.price} />,
    meta: { align: 'right' },
  }),
  col.accessor('gapPercent', {
    header: 'EMA14 vs Smooth10',
    cell: (c) => <Pct value={c.getValue()} />,
    meta: { align: 'right', mobile: 'primary' },
  }),
  col.accessor('createdAt', {
    header: 'Sent',
    cell: (c) => {
      const a = c.row.original
      if (a.sentAt) return <span className="whitespace-nowrap">{formatDateTime(a.sentAt)}</span>
      return (
        <Tooltip>
          <TooltipTrigger asChild>
            <Badge variant="outline" className="border-destructive/50 text-destructive">
              Not sent
            </Badge>
          </TooltipTrigger>
          <TooltipContent>{a.error || 'Discord message failed'}</TooltipContent>
        </Tooltip>
      )
    },
  }),
]

function AlertsPage() {
  const s = Route.useSearch()
  const navigate = useNavigate({ from: Route.fullPath })
  const set = (patch: Partial<z.infer<typeof search>>) =>
    void navigate({ search: (prev) => ({ ...prev, ...patch }), replace: true })
  const { data, isLoading, error, refetch } = useQuery(emaCrossAlertsQuery({ timeframe: s.timeframe }))

  return (
    <>
      <PageHeader
        title="Cross alerts"
        description="Every cross sent to Discord, newest first (kept 90 days)."
      />
      <div className="mb-3 flex flex-col gap-2 sm:flex-row sm:items-center">
        <div className="relative sm:w-72">
          <SearchIcon className="absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Search pairs…"
            className="pl-8"
            value={s.q ?? ''}
            onChange={(e) => set({ q: e.target.value || undefined })}
            aria-label="Search pairs"
          />
        </div>
        <ToggleGroup
          type="single"
          variant="outline"
          size="sm"
          value={s.timeframe ?? 'ALL'}
          onValueChange={(v) => {
            if (v) set({ timeframe: v === 'ALL' ? undefined : (v as (typeof TIMEFRAMES)[number]) })
          }}
        >
          <ToggleGroupItem value="ALL">All</ToggleGroupItem>
          {TIMEFRAMES.map((tf) => (
            <ToggleGroupItem key={tf} value={tf}>
              {tf}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      </div>
      <DataTable
        columns={columns}
        data={data}
        isLoading={isLoading}
        error={error}
        onRetry={() => void refetch()}
        getRowId={(a) => String(a.id)}
        globalFilter={s.q}
        pageSize={50}
        empty={<EmptyState title="No alerts yet" description="Crosses of the watched pairs show up here." />}
      />
    </>
  )
}
