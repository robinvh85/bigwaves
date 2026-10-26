import { useQuery } from '@tanstack/react-query'
import { createColumnHelper } from '@tanstack/react-table'

import { DataTable } from '@/shared/components/data-table'
import { Price } from '@/shared/components/numbers'
import { cn } from '@/shared/lib/utils'

import { type Ema1dSignalRow, signalsQuery } from '../api'
import { engineTerm, engineText } from '../engine-text'

const col = createColumnHelper<Ema1dSignalRow>()
const columns = [
  col.accessor('pair', {
    header: 'Coin',
    cell: (c) => <span className="font-mono font-medium">{c.getValue()}</span>,
    meta: { mobile: 'title' },
  }),
  col.accessor('lenh', {
    header: 'Strategy',
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
        {engineTerm(c.getValue())}
      </span>
    ),
    meta: { mobile: 'badge' },
  }),
  col.accessor('vao', {
    header: 'Entry',
    cell: (c) => (c.getValue() ? engineText(c.getValue() ?? '') : '—'),
  }),
  col.accessor('gia_vao', {
    header: 'Entry price',
    cell: (c) => <Price value={c.getValue() ?? undefined} />,
    meta: { align: 'right' },
  }),
  col.accessor('lai', {
    header: 'Return',
    cell: (c) => <span className="num">{engineText(c.getValue()) || '—'}</span>,
    meta: { align: 'right', mobile: 'primary' },
  }),
  col.accessor('thoat_theo', { header: 'Exit rule' }),
  col.accessor('ema_open', {
    header: 'EMA',
    cell: (c) =>
      `${engineTerm(c.getValue())}${c.row.original.cat_hom_nay !== '-' ? ` · crossed ${engineTerm(c.row.original.cat_hom_nay).toLowerCase()}` : ''}`,
  }),
  col.accessor('hanh_dong_hom_nay', {
    header: 'Today',
    cell: (c) => (
      <span className={cn(c.getValue() !== '-' && 'font-medium text-primary')}>
        {engineText(c.getValue())}
      </span>
    ),
  }),
  col.accessor('ghi_chu', {
    header: 'Notes',
    cell: (c) => <span className="text-xs text-muted-foreground">{engineText(c.getValue())}</span>,
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
          Candle {data.date}
          {data.warnings.length > 0 && (
            <span className="text-warning"> · {data.warnings.map(engineText).join('; ')}</span>
          )}
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
