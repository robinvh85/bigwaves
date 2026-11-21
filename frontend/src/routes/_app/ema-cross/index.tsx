import { useQuery } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { createColumnHelper } from '@tanstack/react-table'
import { PlayIcon, PlusIcon } from 'lucide-react'
import { useState } from 'react'
import { z } from 'zod'

import { CHECK_JOB, type EmaCrossPair, emaCrossConfigQuery, emaCrossPairsQuery } from '@/features/emacross/api'
import { TrendCell } from '@/features/emacross/components/badges'
import { WatchDialog } from '@/features/emacross/components/WatchDialog'
import { useRunJob } from '@/features/jobs/api'
import { PairStatusBadge } from '@/features/pairs/PairStatusBadge'
import { DataTable } from '@/shared/components/data-table'
import { Price } from '@/shared/components/numbers'
import { EmptyState, PageHeader } from '@/shared/components/states'
import { Alert, AlertDescription } from '@/shared/ui/alert'
import { Button } from '@/shared/ui/button'

export const Route = createFileRoute('/_app/ema-cross/')({
  staticData: { title: 'Watchlist' },
  validateSearch: z.object({ pair: z.number().int().optional() }),
  component: WatchlistPage,
})

const col = createColumnHelper<EmaCrossPair>()

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
  col.display({
    id: 'd1',
    header: 'D1 trend',
    cell: (c) => <TrendCell state={c.row.original.state} lastAlert={c.row.original.lastAlert} />,
  }),
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
        description="Discord alert when EMA14 of the close crosses Smooth10 (EMA10 of the EMA) on D1. Checked at minute 03 of every hour; each cross is sent once per candle. No orders are placed."
        actions={
          <>
            <Button variant="outline" disabled={run.isPending} onClick={() => run.mutate(CHECK_JOB)}>
              <PlayIcon />
              Check now
            </Button>
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
            description="Add a pair to get its D1 cross alerts on Discord."
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
