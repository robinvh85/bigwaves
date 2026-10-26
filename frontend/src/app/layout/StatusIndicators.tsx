import { useQuery } from '@tanstack/react-query'
import { ShieldOffIcon } from 'lucide-react'

import { useStreamStore } from '@/app/event-stream'
import { systemQuery } from '@/features/system/api'
import { cn } from '@/shared/lib/utils'
import { Badge } from '@/shared/ui/badge'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/shared/ui/tooltip'

/** Always-visible safety signals: is trading live, is the live update stream connected. */
export function StatusIndicators() {
  const { data: system } = useQuery(systemQuery)
  const connected = useStreamStore((s) => s.connected)

  return (
    <div className="flex items-center gap-2">
      {system && !system.tradingEnabled && (
        <Tooltip>
          <TooltipTrigger asChild>
            <Badge variant="outline" className="gap-1 border-warning/50 text-warning">
              <ShieldOffIcon className="size-3" />
              <span className="sr-only sm:not-sr-only">Read-only</span>
            </Badge>
          </TooltipTrigger>
          <TooltipContent>TRADING_ENABLED is off: no orders are sent to Binance</TooltipContent>
        </Tooltip>
      )}
      <Tooltip>
        <TooltipTrigger asChild>
          <span
            role="status"
            aria-label={connected ? 'Receiving live updates' : 'Live updates disconnected'}
            className={cn(
              'inline-block size-2 rounded-full',
              connected ? 'bg-success' : 'bg-muted-foreground/50',
            )}
          />
        </TooltipTrigger>
        <TooltipContent>{connected ? 'Receiving live updates' : 'Disconnected, retrying…'}</TooltipContent>
      </Tooltip>
    </div>
  )
}
