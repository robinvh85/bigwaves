import { Badge } from '@/shared/ui/badge'

import type { EmaSetting, EmaSettingTrade } from '../api'

const SETTING: Record<EmaSetting['status'], { label: string; className: string }> = {
  ACTIVE: { label: 'Active', className: 'border-success/40 text-success' },
  INACTIVE: { label: 'Paused', className: 'border-warning/50 text-warning' },
  DONE: { label: 'Done', className: 'text-muted-foreground' },
}

export function SettingStatusBadge({ status }: { status: EmaSetting['status'] }) {
  const s = SETTING[status]
  return (
    <Badge variant="outline" className={s.className}>
      {s.label}
    </Badge>
  )
}

const TRADE: Record<EmaSettingTrade['status'], { label: string; className: string }> = {
  OPENING: { label: 'Waiting', className: 'border-primary/40' },
  OPEN: { label: 'Holding', className: 'border-long/50 text-long' },
  CLOSED: { label: 'Closed', className: 'text-muted-foreground' },
}

export function TradeStatusBadge({ status }: { status: EmaSettingTrade['status'] }) {
  const s = TRADE[status]
  return (
    <Badge variant="outline" className={s.className}>
      {s.label}
    </Badge>
  )
}
