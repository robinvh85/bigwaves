import { createColumnHelper } from '@tanstack/react-table'

import { Price, Qty } from '@/shared/components/numbers'
import { fromNow } from '@/shared/lib/date'
import { formatPct } from '@/shared/lib/format'

import type { Pair } from './api'
import { AccountTypeLabel, PairStatusBadge } from './PairStatusBadge'

const col = createColumnHelper<Pair>()

const num = (v: string | undefined) => (v ? Number(v) : -Infinity)

export const pairColumns = [
  col.accessor('name', {
    header: 'Pair',
    cell: (c) => <span className="font-mono font-medium">{c.getValue()}</span>,
    meta: { mobile: 'title' },
  }),
  col.accessor('status', {
    header: 'Trạng thái',
    cell: (c) => <PairStatusBadge status={c.getValue()} />,
    meta: { mobile: 'badge' },
  }),
  col.accessor((p) => num(p.currentPrice), {
    id: 'price',
    header: 'Giá',
    cell: (c) => <Price value={c.row.original.currentPrice} />,
    meta: { align: 'right', mobile: 'primary' },
  }),
  col.accessor((p) => num(p.interestRate), {
    id: 'interest',
    header: 'Lãi vay/năm',
    cell: (c) =>
      c.row.original.interestRate ? formatPct(c.row.original.interestRate, { sign: false }) : '—',
    meta: { align: 'right' },
  }),
  col.accessor((p) => num(p.stepSize), {
    id: 'step',
    header: 'Step',
    cell: (c) => <Qty value={c.row.original.stepSize} />,
    meta: { align: 'right' },
  }),
  col.display({
    id: 'account',
    header: 'Tài khoản',
    cell: (c) => <AccountTypeLabel isSpot={c.row.original.isSpot} isIsolated={c.row.original.isIsolated} />,
  }),
  col.accessor((p) => p.priceUpdatedAt ?? '', {
    id: 'updated',
    header: 'Cập nhật giá',
    cell: (c) => <span className="text-muted-foreground">{fromNow(c.row.original.priceUpdatedAt)}</span>,
    meta: { align: 'right', mobile: 'hidden' },
  }),
]
