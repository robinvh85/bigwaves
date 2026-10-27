import { TrendingDownIcon, TrendingUpIcon } from 'lucide-react'

import { Pct } from '@/shared/components/numbers'
import { fromNow } from '@/shared/lib/date'
import { cn } from '@/shared/lib/utils'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/shared/ui/tooltip'

import type { EmaCrossAlert, EmaCrossState } from '../api'

/** UP / DOWN with an arrow, in the long / short color. */
export function Direction({ direction, className }: { direction: 'UP' | 'DOWN'; className?: string }) {
  const Icon = direction === 'UP' ? TrendingUpIcon : TrendingDownIcon
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 text-xs font-semibold',
        direction === 'UP' ? 'text-long' : 'text-short',
        className,
      )}
    >
      <Icon className="size-3.5" />
      {direction}
    </span>
  )
}

/** One timeframe of a watched pair: the trend, the EMA14 / Smooth10 gap and the last cross. */
export function TimeframeCell({ state, lastAlert }: { state?: EmaCrossState; lastAlert?: EmaCrossAlert }) {
  if (!state) return <span className="text-xs text-muted-foreground">Not checked yet</span>
  return (
    <div className="flex flex-col gap-0.5">
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="inline-flex items-center gap-2">
            <Direction direction={state.trend} />
            <Pct value={state.gapPercent} className="text-xs" />
          </span>
        </TooltipTrigger>
        <TooltipContent>EMA14 vs Smooth10, checked {fromNow(state.checkedAt)}</TooltipContent>
      </Tooltip>
      {lastAlert && (
        <span className="text-xs text-muted-foreground">
          crossed {lastAlert.direction.toLowerCase()} {fromNow(lastAlert.candleTime)}
        </span>
      )}
    </div>
  )
}
