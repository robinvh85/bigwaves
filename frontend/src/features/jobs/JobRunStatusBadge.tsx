import { Loader2Icon } from 'lucide-react'

import { Badge } from '@/shared/ui/badge'

import type { JobRun } from './api'

const STYLE: Record<JobRun['status'], { label: string; className: string }> = {
  OK: { label: 'OK', className: 'border-success/40 text-success' },
  FAILED: { label: 'Lỗi', className: 'border-destructive/50 text-destructive' },
  SKIPPED: { label: 'Bỏ qua', className: 'text-muted-foreground' },
  RUNNING: { label: 'Đang chạy', className: 'border-primary/40' },
}

export function JobRunStatusBadge({ status }: { status: JobRun['status'] }) {
  const s = STYLE[status]
  return (
    <Badge variant="outline" className={s.className}>
      {status === 'RUNNING' && <Loader2Icon className="size-3 animate-spin" />}
      {s.label}
    </Badge>
  )
}
