import { useQuery } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { createColumnHelper } from '@tanstack/react-table'
import { PlayIcon, PlusIcon } from 'lucide-react'
import { useState } from 'react'
import { z } from 'zod'

import {
  checkJob,
  type EmaCrossPair,
  emaCrossConfigQuery,
  emaCrossPairsQuery,
  TIMEFRAMES,
  type Timeframe,
  watchKey,
} from '@/features/emacross/api'
import { TimeframeCell } from '@/features/emacross/components/badges'
import { WatchDialog } from '@/features/emacross/components/WatchDialog'
import { useRunJob } from '@/features/jobs/api'
import { PairStatusBadge } from '@/features/pairs/PairStatusBadge'
import { DataTable } from '@/shared/components/data-table'
import { Price } from '@/shared/components/numbers'
import { EmptyState, PageHeader } from '@/shared/components/states'
import { Alert, AlertDescription } from '@/shared/ui/alert'
import { Button } from '@/shared/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/shared/ui/dropdown-menu'

export const Route = createFileRoute('/_app/ema-cross/')({
  staticData: { title: 'Watchlist' },
  validateSearch: z.object({ pair: z.number().int().optional() }),
  component: WatchlistPage,
})

const col = createColumnHelper<EmaCrossPair>()

function timeframeColumn(tf: Timeframe) {
  return col.display({
    id: tf,
    header: tf,
    cell: (c) => {
      const p = c.row.original
      if (!p[watchKey(tf)]) return <span className="text-muted-foreground">—</span>
      return (
        <TimeframeCell
          state={p.states.find((s) => s.timeframe === tf)}
          lastAlert={p.lastAlerts.find((a) => a.timeframe === tf)}
        />
      )
    },
  })
}

const columns = [
  col.accessor('pairName', {
    header: 'Pair',
    cell: (c) => <span className="font-mono font-medium">{c.getValue()}</span>,
    meta: { mobile: 'title' },
  }),
  col.accessor('pairStatus', {
    header: 'Status',
    cell: (c) => <PairStatusBadge status={c.getValue()} />,
    meta: { mobile: 'badge' },
  }),
  ...TIMEFRAMES.map(timeframeColumn),
  col.accessor((p) => (p.currentPrice ? Number(p.currentPrice) : 0), {
    id: 'price',
    header: 'Price',
    cell: (c) => <Price value={c.row.original.currentPrice} updatedAt={c.row.original.priceUpdatedAt} />,
    meta: { align: 'right', mobile: 'primary' },
  }),
]

function WatchlistPage() {
  const { pair } = Route.useSearch()
  const navigate = useNavigate({ from: Route.fullPath })
  const { data, isLoading, error, refetch } = useQuery(emaCrossPairsQuery)
  const { data: config } = useQuery(emaCrossConfigQuery)
  const run = useRunJob()
  const [adding, setAdding] = useState(false)
  const editing = pair === undefined ? undefined : data?.find((p) => p.pairId === pair)

  return (
    <>
      <PageHeader
        title="EMA Cross"
        description="Discord alert when EMA14 of the close crosses Smooth10 (EMA10 of the EMA). H1 is checked at minutes 01 and 31, H4 and D1 at minute 03 of every hour; each cross is sent once per candle. No orders are placed."
        actions={
          <>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="outline" disabled={run.isPending}>
                  <PlayIcon />
                  Check now
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                {TIMEFRAMES.map((tf) => (
                  <DropdownMenuItem key={tf} onSelect={() => run.mutate(checkJob(tf))}>
                    {tf}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
            <Button onClick={() => setAdding(true)}>
              <PlusIcon />
              Watch a pair
            </Button>
          </>
        }
      />
      {config && !config.config.enabled && (
        <Alert className="mb-4">
          <AlertDescription>Alerts are switched off in Settings: the checks skip.</AlertDescription>
        </Alert>
      )}
      <DataTable
        columns={columns}
        data={data}
        isLoading={isLoading}
        error={error}
        onRetry={() => void refetch()}
        getRowId={(p) => String(p.pairId)}
        onRowClick={(p) => void navigate({ search: { pair: p.pairId }, replace: true })}
        rowClassName={(p) => (p.pairStatus === 'DISABLED' ? 'opacity-60' : undefined)}
        empty={
          <EmptyState
            title="No pairs watched"
            description="Add a pair and pick the timeframes to get cross alerts on Discord."
          />
        }
      />
      <WatchDialog open={adding} onOpenChange={setAdding} watched={data?.map((p) => p.pairId) ?? []} />
      <WatchDialog
        open={editing !== undefined}
        onOpenChange={(o) => !o && void navigate({ search: {}, replace: true })}
        pair={editing}
        watched={[]}
      />
    </>
  )
}
