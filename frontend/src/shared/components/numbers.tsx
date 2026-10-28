import type { ReactNode } from 'react'

import {
  type DecimalString,
  formatMoney,
  formatPct,
  formatPrice,
  formatQty,
  signOf,
} from '@/shared/lib/format'
import { formatDateTime, fromNow } from '@/shared/lib/date'
import { cn } from '@/shared/lib/utils'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/shared/ui/tooltip'

type Value = DecimalString | number | null | undefined

function Num({
  text,
  full,
  note,
  className,
}: {
  text: string
  full?: string
  /** Extra line in the tooltip, under the full value. */
  note?: ReactNode
  className?: string
}) {
  const span = <span className={cn('num whitespace-nowrap', className)}>{text}</span>
  const showFull = !!full && full !== text
  if (!showFull && !note) return span
  return (
    <Tooltip>
      <TooltipTrigger asChild>{span}</TooltipTrigger>
      <TooltipContent>
        {showFull && <div className="num">{full}</div>}
        {note && <div>{note}</div>}
      </TooltipContent>
    </Tooltip>
  )
}

/** USDT amount. Hover shows the full precision. */
export function Money({
  value,
  unit = 'USDT',
  className,
}: {
  value: Value
  unit?: string | null
  className?: string
}) {
  const text = formatMoney(value)
  return (
    <Num
      text={unit && text !== '—' ? `${text} ${unit}` : text}
      full={value !== null && value !== undefined ? String(value) : undefined}
      className={className}
    />
  )
}

/**
 * A price. `updatedAt` (when the price was fetched) shows on hover; `exact` prints the value as
 * stored instead of rounding it, for forms that ask for prices.
 */
export function Price({
  value,
  updatedAt,
  exact,
  className,
}: {
  value: Value
  updatedAt?: string | null
  exact?: boolean
  className?: string
}) {
  const full = value != null ? String(value) : undefined
  return (
    <Num
      text={exact && full !== undefined ? full : formatPrice(value)}
      full={full}
      note={
        updatedAt && value != null
          ? `Updated ${fromNow(updatedAt)} · ${formatDateTime(updatedAt)}`
          : undefined
      }
      className={className}
    />
  )
}

export function Qty({ value, stepSize, className }: { value: Value; stepSize?: string; className?: string }) {
  return (
    <Num
      text={formatQty(value, stepSize)}
      full={value != null ? String(value) : undefined}
      className={className}
    />
  )
}

const PNL_COLOR = { [-1]: 'text-pnl-down', 0: 'text-muted-foreground', 1: 'text-pnl-up' } as const

/** Profit/loss amount colored by sign, with an explicit +/- so color is not the only signal. */
export function Pnl({
  value,
  unit = 'USDT',
  className,
}: {
  value: Value
  unit?: string | null
  className?: string
}) {
  const s = signOf(value)
  const text = formatMoney(value)
  const signed = s > 0 ? `+${text}` : text
  return (
    <Num
      text={unit && text !== '—' ? `${signed} ${unit}` : signed}
      full={value != null ? String(value) : undefined}
      className={cn(PNL_COLOR[s], className)}
    />
  )
}

export function Pct({
  value,
  ratio,
  className,
  colored = true,
}: {
  value: Value
  ratio?: boolean
  className?: string
  colored?: boolean
}) {
  return (
    <Num text={formatPct(value, { ratio })} className={cn(colored && PNL_COLOR[signOf(value)], className)} />
  )
}

/** LONG/SHORT label: colored and spelled out. */
export function Side({ side, className }: { side: 'LONG' | 'SHORT'; className?: string }) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded px-1.5 py-0.5 text-xs font-semibold',
        side === 'LONG' ? 'bg-long/15 text-long' : 'bg-short/15 text-short',
        className,
      )}
    >
      {side}
    </span>
  )
}
