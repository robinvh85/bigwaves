import { createColumnHelper } from '@tanstack/react-table'

import { Pct, Pnl, Price, Qty, Side } from '@/shared/components/numbers'
import { formatDate } from '@/shared/lib/date'
import { formatMoney } from '@/shared/lib/format'

import type { Ema1dPosition } from '../api'
import { Slippage } from './Slippage'

const col = createColumnHelper<Ema1dPosition>()
const num = (v?: string) => (v ? Number(v) : -Infinity)

const base = [
  col.accessor('coin', {
    header: 'Coin',
    cell: (c) => <span className="font-mono font-medium">{c.getValue()}</span>,
    meta: { mobile: 'title' },
  }),
  col.accessor('side', {
    header: 'Chiều',
    cell: (c) => <Side side={c.getValue()} />,
    meta: { mobile: 'badge' },
  }),
  col.accessor('signalDate', { header: 'Tín hiệu', cell: (c) => formatDate(c.getValue()) }),
  col.accessor((p) => num(p.openPrice), {
    id: 'open',
    header: 'Giá vào',
    cell: (c) => <Price value={c.row.original.openPrice} />,
    meta: { align: 'right' },
  }),
  col.display({
    id: 'openSlip',
    header: 'Trượt vào',
    cell: (c) => <Slippage value={c.row.original.openSlippagePercent} />,
    meta: { align: 'right', mobile: 'hidden' },
  }),
]

export const openPositionColumns = [
  ...base,
  col.accessor((p) => num(p.currentPrice), {
    id: 'current',
    header: 'Giá hiện tại',
    cell: (c) => <Price value={c.row.original.currentPrice} />,
    meta: { align: 'right' },
  }),
  col.accessor((p) => num(p.notionalUsdt), {
    id: 'notional',
    header: 'Vốn',
    cell: (c) => (
      <span className="num">
        {formatMoney(c.row.original.notionalUsdt)}{' '}
        <span className="text-xs text-muted-foreground">({c.row.original.sizePercent}%)</span>
      </span>
    ),
    meta: { align: 'right', mobile: 'hidden' },
  }),
  col.accessor((p) => num(p.profitUsdt), {
    id: 'pnl',
    header: 'Lãi/lỗ',
    cell: (c) => (
      <div className="flex flex-col items-end">
        <Pnl value={c.row.original.profitUsdt} />
        <Pct value={c.row.original.profitPercent} className="text-xs" />
      </div>
    ),
    meta: { align: 'right', mobile: 'primary' },
  }),
  col.accessor((p) => p.daysHeld ?? -1, {
    id: 'days',
    header: 'Ngày giữ',
    cell: (c) => <span className="num">{c.row.original.daysHeld ?? '—'}</span>,
    meta: { align: 'right' },
  }),
]

export const closedPositionColumns = [
  ...base,
  col.accessor((p) => num(p.closePrice), {
    id: 'close',
    header: 'Giá ra',
    cell: (c) => <Price value={c.row.original.closePrice} />,
    meta: { align: 'right' },
  }),
  col.display({
    id: 'closeSlip',
    header: 'Trượt ra',
    cell: (c) => <Slippage value={c.row.original.closeSlippagePercent} />,
    meta: { align: 'right', mobile: 'hidden' },
  }),
  col.accessor('amount', {
    header: 'Khối lượng',
    cell: (c) => <Qty value={c.getValue()} />,
    meta: { align: 'right', mobile: 'hidden' },
  }),
  col.accessor((p) => num(p.feeUsdt) + num(p.interestUsdt), {
    id: 'costs',
    header: 'Phí + lãi vay',
    cell: (c) => (
      <span className="num text-muted-foreground">
        {formatMoney(Number(c.row.original.feeUsdt) + Number(c.row.original.interestUsdt))}
      </span>
    ),
    meta: { align: 'right', mobile: 'hidden' },
  }),
  col.accessor((p) => num(p.profitUsdt), {
    id: 'pnl',
    header: 'Lãi/lỗ',
    cell: (c) => (
      <div className="flex flex-col items-end">
        <Pnl value={c.row.original.profitUsdt} />
        <Pct value={c.row.original.profitPercent} className="text-xs" />
      </div>
    ),
    meta: { align: 'right', mobile: 'primary' },
  }),
  col.accessor((p) => p.closeAt ?? '', {
    id: 'closedAt',
    header: 'Đóng lúc',
    cell: (c) => formatDate(c.row.original.closeAt),
    meta: { align: 'right' },
  }),
]
