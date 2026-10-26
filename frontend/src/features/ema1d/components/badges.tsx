import { Loader2Icon } from 'lucide-react'

import { cn } from '@/shared/lib/utils'
import { Badge } from '@/shared/ui/badge'

import type { Ema1dPairRow, Ema1dTodo } from '../api'

const TODO: Record<Ema1dTodo['status'], { label: string; className: string }> = {
  PENDING: { label: 'Pending', className: 'border-primary/40' },
  RUNNING: { label: 'Running', className: 'border-primary/60' },
  DONE: { label: 'Xong', className: 'border-success/40 text-success' },
  FAILED: { label: 'Failed', className: 'border-destructive/50 text-destructive' },
  SKIPPED: { label: 'Skipped', className: 'border-warning/50 text-warning' },
  CANCELLED: { label: 'Cancelled', className: 'text-muted-foreground' },
}

export function TodoStatusBadge({ status }: { status: Ema1dTodo['status'] }) {
  const s = TODO[status]
  return (
    <Badge variant="outline" className={s.className}>
      {status === 'RUNNING' && <Loader2Icon className="size-3 animate-spin" />}
      {s.label}
    </Badge>
  )
}

const PAIR: Record<Ema1dPairRow['status'], { label: string; className: string }> = {
  ACTIVE: { label: 'In basket', className: 'border-success/40 text-success' },
  EXIT_ONLY: { label: 'Exit only', className: 'border-warning/50 text-warning' },
  DISABLED: { label: 'Disabled', className: 'text-muted-foreground' },
}

export function BasketStatusBadge({ status }: { status: Ema1dPairRow['status'] }) {
  const s = PAIR[status]
  return (
    <Badge variant="outline" className={s.className}>
      {s.label}
    </Badge>
  )
}

export function GroupBadge({ group }: { group: Ema1dPairRow['pairGroup'] }) {
  return (
    <span
      className={cn('text-xs font-medium', group === 'CORE' ? 'text-foreground' : 'text-muted-foreground')}
    >
      {group === 'CORE' ? 'Core' : 'Sensitive'}
    </span>
  )
}

/** L / S / F as wanted by the strategy. */
export function WantedLabel({ state }: { state: 'L' | 'S' | 'F' }) {
  if (state === 'F') return <span className="text-muted-foreground">Flat</span>
  return (
    <span className={cn('font-medium', state === 'L' ? 'text-long' : 'text-short')}>
      {state === 'L' ? 'LONG' : 'SHORT'}
    </span>
  )
}
