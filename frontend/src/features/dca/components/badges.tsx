import { Badge } from '@/shared/ui/badge'

import type { DcaSetting, DcaTrade } from '../api'

const SETTING: Record<DcaSetting['status'], { label: string; className: string }> = {
  ACTIVE: { label: 'Active', className: 'border-success/40 text-success' },
  INACTIVE: { label: 'Paused', className: 'border-warning/50 text-warning' },
  DONE: { label: 'Xong', className: 'text-muted-foreground' },
}

export function SettingStatusBadge({ status }: { status: DcaSetting['status'] }) {
  const s = SETTING[status]
  return (
    <Badge variant="outline" className={s.className}>
      {s.label}
    </Badge>
  )
}

const TRADE: Record<DcaTrade['status'], { label: string; className: string }> = {
  OPENING: { label: 'Waiting to buy', className: 'border-primary/40' },
  OPEN: { label: 'Holding', className: 'border-long/50 text-long' },
  CLOSED: { label: 'Sold', className: 'text-muted-foreground' },
}

export function TradeStatusBadge({ status }: { status: DcaTrade['status'] }) {
  const s = TRADE[status]
  return (
    <Badge variant="outline" className={s.className}>
      {s.label}
    </Badge>
  )
}
