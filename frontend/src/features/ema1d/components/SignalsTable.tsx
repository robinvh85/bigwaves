import { useQuery } from '@tanstack/react-query'
import { createColumnHelper } from '@tanstack/react-table'

import { DataTable } from '@/shared/components/data-table'
import { Price } from '@/shared/components/numbers'
import { cn } from '@/shared/lib/utils'

import { type Ema1dSignalRow, signalsQuery } from '../api'

const col = createColumnHelper<Ema1dSignalRow>()
const columns = [
  col.accessor('pair', {
    header: 'Coin',
    cell: (c) => <span className="font-mono font-medium">{c.getValue()}</span>,
    meta: { mobile: 'title' },
  }),
  col.accessor('lenh', {
    header: 'Chiến lược',
    cell: (c) => (
      <span
        className={cn(
          'font-medium',
          c.getValue() === 'LONG'
            ? 'text-long'
            : c.getValue() === 'SHORT'
              ? 'text-short'
              : 'text-muted-foreground',
        )}
      >
        {c.getValue()}
      </span>
    ),
    meta: { mobile: 'badge' },
  }),
  col.accessor('vao', { header: 'Vào', cell: (c) => c.getValue() ?? '—' }),
  col.accessor('gia_vao', {
    header: 'Giá vào',
    cell: (c) => <Price value={c.getValue() ?? undefined} />,
    meta: { align: 'right' },
  }),
  col.accessor('lai', {
    header: 'Lãi',
    cell: (c) => <span className="num">{c.getValue() || '—'}</span>,
    meta: { align: 'right', mobile: 'primary' },
  }),
  col.accessor('thoat_theo', { header: 'Thoát theo' }),
  col.accessor('ema_open', {
    header: 'EMA',
    cell: (c) =>
      `${c.getValue()}${c.row.original.cat_hom_nay !== '-' ? ` · cắt ${c.row.original.cat_hom_nay}` : ''}`,
  }),
  col.accessor('hanh_dong_hom_nay', {
    header: 'Hôm nay',
    cell: (c) => (
      <span className={cn(c.getValue() !== '-' && 'font-medium text-primary')}>{c.getValue()}</span>
    ),
  }),
  col.accessor('ghi_chu', {
    header: 'Ghi chú',
    cell: (c) => <span className="text-xs text-muted-foreground">{c.getValue()}</span>,
    meta: { mobile: 'hidden' },
  }),
]

/** The strategy table computed from the stored candles: what the next sync would ask for. */
export function SignalsTable() {
  const { data, isLoading, error, refetch } = useQuery(signalsQuery)
  return (
    <div className="space-y-2">
      {data && (
        <p className="text-sm text-muted-foreground">
          Nến {data.date}
          {data.warnings.length > 0 && <span className="text-warning"> · {data.warnings.join('; ')}</span>}
        </p>
      )}
      <DataTable
        columns={columns}
        data={data?.rows}
        isLoading={isLoading}
        error={error}
        onRetry={() => void refetch()}
        getRowId={(r) => r.pair}
      />
    </div>
  )
}
