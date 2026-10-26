import type { ReactNode } from 'react'

import { cn } from '@/shared/lib/utils'
import { Card, CardContent } from '@/shared/ui/card'
import { Skeleton } from '@/shared/ui/skeleton'

export function StatCard({
  label,
  value,
  hint,
  loading,
  className,
}: {
  label: string
  value: ReactNode
  hint?: ReactNode
  loading?: boolean
  className?: string
}) {
  return (
    <Card className={cn('gap-1 py-4', className)}>
      <CardContent className="space-y-1 px-4">
        <p className="text-xs font-medium text-muted-foreground uppercase">{label}</p>
        {loading ? (
          <Skeleton className="h-7 w-28" />
        ) : (
          <div className="text-xl font-semibold md:text-2xl">{value}</div>
        )}
        {hint && <div className="text-xs text-muted-foreground">{hint}</div>}
      </CardContent>
    </Card>
  )
}
