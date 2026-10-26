import { cn } from '@/shared/lib/utils'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/shared/ui/tooltip'

/** Positive slippage is always worse for us (paid more / received less than the strategy price). */
export function Slippage({ value }: { value?: string }) {
  if (!value) return <span className="text-muted-foreground">—</span>
  const v = Number(value)
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          className={cn('num', v > 0.5 ? 'text-pnl-down' : v < 0 ? 'text-pnl-up' : 'text-muted-foreground')}
        >
          {v > 0 ? '+' : ''}
          {v.toFixed(2)}%
        </span>
      </TooltipTrigger>
      <TooltipContent>Trượt giá so với giá chiến lược (dương = bất lợi)</TooltipContent>
    </Tooltip>
  )
}
