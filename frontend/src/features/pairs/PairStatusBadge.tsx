import { Badge } from '@/shared/ui/badge'

import type { PairStatus } from './api'

export function PairStatusBadge({ status }: { status: PairStatus }) {
  return status === 'ACTIVE' ? (
    <Badge variant="outline" className="border-success/40 text-success">
      Active
    </Badge>
  ) : (
    <Badge variant="outline" className="text-muted-foreground">
      Disabled
    </Badge>
  )
}

export function AccountTypeLabel({ isSpot, isIsolated }: { isSpot: boolean; isIsolated: boolean }) {
  return <span className="text-muted-foreground">{isSpot ? 'Spot' : isIsolated ? 'Isolated' : 'Cross'}</span>
}
