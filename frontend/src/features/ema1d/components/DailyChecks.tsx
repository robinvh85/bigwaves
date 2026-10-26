import { useQuery } from '@tanstack/react-query'
import { ChevronDownIcon, Loader2Icon, PlayIcon } from 'lucide-react'
import { useState } from 'react'

import { useRunJob } from '@/features/jobs/api'
import { Markdown } from '@/shared/components/markdown'
import { EmptyState, ErrorState, SkeletonRows } from '@/shared/components/states'
import { formatDate, formatDateTime } from '@/shared/lib/date'
import { cn } from '@/shared/lib/utils'
import { Badge } from '@/shared/ui/badge'
import { Button } from '@/shared/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/shared/ui/collapsible'

import { dailyChecksQuery, type Ema1dDailyCheck } from '../api'

const STATUS: Record<Ema1dDailyCheck['status'], string> = {
  PENDING: 'text-muted-foreground',
  SYNCED: 'border-warning/50 text-warning',
  SENT: 'border-success/40 text-success',
  FAILED: 'border-destructive/50 text-destructive',
}

function CheckRow({ c }: { c: Ema1dDailyCheck }) {
  const [open, setOpen] = useState(false)
  return (
    <Collapsible open={open} onOpenChange={setOpen} className="rounded-lg border">
      <CollapsibleTrigger className="flex w-full items-center gap-3 p-3 text-left">
        <ChevronDownIcon className={cn('size-4 shrink-0 transition-transform', open && 'rotate-180')} />
        <span className="font-medium">{formatDate(c.signalDate)}</span>
        <Badge variant="outline" className={STATUS[c.status]}>
          {c.status}
        </Badge>
        <span className="ml-auto text-xs text-muted-foreground">
          {c.sentAt ? `gửi ${formatDateTime(c.sentAt)}` : c.attempts > 1 ? `${c.attempts} lần chạy` : ''}
        </span>
      </CollapsibleTrigger>
      <CollapsibleContent className="border-t p-3">
        {c.lastError && <p className="mb-3 text-sm text-destructive">{c.lastError}</p>}
        {c.report ? (
          <Markdown className="prose-p:my-1">{c.report.replaceAll('\n', '  \n')}</Markdown>
        ) : (
          <p className="text-sm text-muted-foreground">Chưa có báo cáo.</p>
        )}
      </CollapsibleContent>
    </Collapsible>
  )
}

export function DailyChecks() {
  const { data, isLoading, error, refetch } = useQuery(dailyChecksQuery)
  const run = useRunJob()
  return (
    <div className="space-y-2">
      <div className="flex justify-end">
        <Button
          variant="outline"
          size="sm"
          disabled={run.isPending}
          onClick={() => run.mutate('ema1d.daily_check')}
        >
          {run.isPending ? <Loader2Icon className="animate-spin" /> : <PlayIcon />}
          Chạy daily check ngay
        </Button>
      </div>
      {error ? (
        <ErrorState error={error} onRetry={() => void refetch()} />
      ) : isLoading || !data ? (
        <SkeletonRows rows={4} />
      ) : data.length === 0 ? (
        <EmptyState title="Chưa có daily check" description="Bật daily check trong cấu hình hoặc chạy tay." />
      ) : (
        data.map((c) => <CheckRow key={c.signalDate} c={c} />)
      )}
    </div>
  )
}
