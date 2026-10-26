import { useQuery } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { z } from 'zod'

import { pairsQuery, tradesQuery } from '@/features/ema1d/api'
import { closedPositionColumns, openPositionColumns } from '@/features/ema1d/components/positionColumns'
import { DataTable } from '@/shared/components/data-table'
import { Money, Pnl } from '@/shared/components/numbers'
import { EmptyState, PageHeader } from '@/shared/components/states'
import { StatCard } from '@/shared/components/stat-card'
import { formatPct } from '@/shared/lib/format'
import { Input } from '@/shared/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui/select'
import { ToggleGroup, ToggleGroupItem } from '@/shared/ui/toggle-group'

const search = z.object({
  status: z.enum(['open', 'closed']).optional(),
  coin: z.string().optional(),
  side: z.enum(['LONG', 'SHORT']).optional(),
  month: z.string().optional(),
})

export const Route = createFileRoute('/_app/ema1d/trades')({
  staticData: { title: 'Lịch sử lệnh' },
  validateSearch: search,
  component: TradesPage,
})

const ALL = '__all'

function TradesPage() {
  const s = Route.useSearch()
  const navigate = useNavigate({ from: Route.fullPath })
  const set = (patch: Partial<z.infer<typeof search>>) =>
    void navigate({ search: (p) => ({ ...p, ...patch }), replace: true })
  const status = s.status ?? 'closed'
  const { data, isLoading, error, refetch } = useQuery(
    tradesQuery({ status, coin: s.coin, side: s.side, month: s.month }),
  )
  const { data: pairs } = useQuery(pairsQuery)
  const sum = data?.summary

  return (
    <>
      <PageHeader
        title="Lịch sử lệnh"
        description="Lãi/lỗ đã trừ phí hai chiều và lãi vay (SHORT). Trượt giá so với giá open của nến tín hiệu."
      />
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <ToggleGroup
          type="single"
          variant="outline"
          size="sm"
          value={status}
          onValueChange={(v) => v && set({ status: v as 'open' | 'closed' })}
        >
          <ToggleGroupItem value="closed">Đã đóng</ToggleGroupItem>
          <ToggleGroupItem value="open">Đang mở</ToggleGroupItem>
        </ToggleGroup>
        <Select value={s.coin ?? ALL} onValueChange={(v) => set({ coin: v === ALL ? undefined : v })}>
          <SelectTrigger size="sm" className="w-32" aria-label="Lọc coin">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>Mọi coin</SelectItem>
            {pairs?.map((p) => (
              <SelectItem key={p.id} value={p.coin}>
                {p.coin}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select
          value={s.side ?? ALL}
          onValueChange={(v) => set({ side: v === ALL ? undefined : (v as 'LONG' | 'SHORT') })}
        >
          <SelectTrigger size="sm" className="w-28" aria-label="Lọc chiều">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>Hai chiều</SelectItem>
            <SelectItem value="LONG">LONG</SelectItem>
            <SelectItem value="SHORT">SHORT</SelectItem>
          </SelectContent>
        </Select>
        <Input
          type="month"
          className="h-8 w-40"
          aria-label="Lọc tháng"
          value={s.month ?? ''}
          onChange={(e) => set({ month: e.target.value || undefined })}
        />
      </div>
      <div className="mb-4 grid grid-cols-2 gap-3 lg:grid-cols-4">
        <StatCard
          label="Số lệnh"
          loading={isLoading}
          value={sum?.count ?? '—'}
          hint={sum?.winRatePercent ? `thắng ${formatPct(sum.winRatePercent, { sign: false })}` : undefined}
        />
        <StatCard label="Tổng lãi/lỗ" loading={isLoading} value={<Pnl value={sum?.totalProfitUsdt} />} />
        <StatCard
          label="TB thắng / thua"
          loading={isLoading}
          value={
            <span className="text-base">
              <Pnl value={sum?.avgWinUsdt} unit={null} /> / <Pnl value={sum?.avgLossUsdt} unit={null} />
            </span>
          }
        />
        <StatCard
          label="Phí + lãi vay"
          loading={isLoading}
          value={<Money value={sum ? String(Number(sum.feeUsdt) + Number(sum.interestUsdt)) : undefined} />}
        />
      </div>
      <DataTable
        columns={status === 'closed' ? closedPositionColumns : openPositionColumns}
        data={data?.items}
        isLoading={isLoading}
        error={error}
        onRetry={() => void refetch()}
        getRowId={(p) => String(p.id)}
        empty={<EmptyState title="Không có lệnh phù hợp bộ lọc" />}
      />
    </>
  )
}
