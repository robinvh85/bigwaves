import { formatPrice } from '@/shared/lib/format'
import { cn } from '@/shared/lib/utils'

import type { DcaSetting, DcaTrade } from '../api'

type Level = { price: number; kind: 'buy' | 'sell' | 'current'; label: string; rung?: DcaTrade }

/**
 * The ladder on one price axis: waiting buys below, targets of held rungs above, and the current
 * price between them, so the next fill is obvious at a glance.
 */
export function PriceLadder({ setting }: { setting: DcaSetting }) {
  const levels: Level[] = []
  for (const t of setting.trades ?? []) {
    if (t.status === 'OPENING')
      levels.push({ price: Number(t.buyPrice), kind: 'buy', label: `mua #${t.id} · ${t.amount}`, rung: t })
    if (t.status === 'OPEN' && Number(t.sellPrice) > 0)
      levels.push({
        price: Number(t.sellPrice),
        kind: 'sell',
        label: `bán #${t.id} · ${t.heldAmount}`,
        rung: t,
      })
  }
  if (setting.currentPrice)
    levels.push({ price: Number(setting.currentPrice), kind: 'current', label: 'giá hiện tại' })
  if (levels.length < 2) return null
  levels.sort((a, b) => b.price - a.price)
  const max = levels[0]!.price
  const min = levels[levels.length - 1]!.price
  const span = max - min || 1

  return (
    <div
      className="relative h-64 rounded-lg border bg-muted/20 px-3"
      role="img"
      aria-label="Bậc thang giá DCA"
    >
      {levels.map((l, i) => {
        const top = ((max - l.price) / span) * 100
        return (
          <div
            key={i}
            className="absolute right-3 left-3 flex -translate-y-1/2 items-center gap-2 text-xs"
            style={{ top: `calc(${top}% * 0.86 + 7%)` }}
          >
            <span
              className={cn(
                'num w-24 shrink-0 text-right font-medium',
                l.kind === 'current' ? 'text-foreground' : l.kind === 'buy' ? 'text-primary' : 'text-long',
              )}
            >
              {formatPrice(l.price)}
            </span>
            <span
              className={cn(
                'h-px flex-1',
                l.kind === 'current'
                  ? 'h-0.5 bg-foreground'
                  : l.kind === 'buy'
                    ? 'border-t border-dashed border-primary'
                    : 'bg-long/60',
              )}
            />
            <span
              className={cn(
                'w-36 shrink-0 truncate',
                l.kind === 'current' ? 'font-semibold' : 'text-muted-foreground',
              )}
            >
              {l.label}
            </span>
          </div>
        )
      })}
    </div>
  )
}
