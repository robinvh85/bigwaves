import { useQuery } from '@tanstack/react-query'
import { ChevronDownIcon, Loader2Icon, PlayIcon } from 'lucide-react'
import { useState } from 'react'

import { EmptyState, ErrorState, SkeletonRows } from '@/shared/components/states'
import { formatDateTime, fromNow } from '@/shared/lib/date'
import { cn } from '@/shared/lib/utils'
import { Button } from '@/shared/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/shared/ui/collapsible'

import { type JobStatus, jobRunsQuery, jobsQuery, useRunJob } from './api'
import { JobRunStatusBadge } from './JobRunStatusBadge'

function duration(start: string, end?: string) {
  if (!end) return ''
  const ms = new Date(end).getTime() - new Date(start).getTime()
  return ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(1)} s`
}

function Runs({ name }: { name: string }) {
  const { data, error, isLoading, refetch } = useQuery(jobRunsQuery(name))
  if (error) return <ErrorState error={error} onRetry={() => void refetch()} />
  if (isLoading || !data) return <SkeletonRows rows={3} />
  if (data.length === 0) return <p className="text-sm text-muted-foreground">Chưa chạy lần nào.</p>
  return (
    <ul className="divide-y text-sm">
      {data.map((r) => (
        <li key={r.id} className="flex flex-col gap-1 py-2 sm:flex-row sm:items-start sm:gap-3">
          <div className="flex shrink-0 items-center gap-2 sm:w-64">
            <JobRunStatusBadge status={r.status} />
            <span className="num text-muted-foreground">{formatDateTime(r.startedAt)}</span>
            <span className="num text-xs text-muted-foreground">{duration(r.startedAt, r.finishedAt)}</span>
          </div>
          <span className={cn('min-w-0 break-words', r.status === 'FAILED' && 'text-destructive')}>
            {r.trigger === 'manual' && <span className="mr-1 text-xs text-muted-foreground">[chạy tay]</span>}
            {r.message || '—'}
          </span>
        </li>
      ))}
    </ul>
  )
}

function JobRow({ job }: { job: JobStatus }) {
  const [open, setOpen] = useState(false)
  const run = useRunJob()
  const busy = job.running || (run.isPending && run.variables === job.name)

  return (
    <Collapsible open={open} onOpenChange={setOpen} className="rounded-lg border bg-card">
      <div className="flex flex-col gap-3 p-3 sm:flex-row sm:items-center">
        <CollapsibleTrigger asChild>
          <button type="button" className="flex min-w-0 flex-1 items-start gap-2 text-left">
            <ChevronDownIcon
              className={cn('mt-0.5 size-4 shrink-0 transition-transform', open && 'rotate-180')}
            />
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-mono text-sm font-medium">{job.name}</span>
                {job.running ? (
                  <JobRunStatusBadge status="RUNNING" />
                ) : (
                  job.lastRun && <JobRunStatusBadge status={job.lastRun.status} />
                )}
              </div>
              <p className="text-sm text-muted-foreground">{job.description}</p>
              {job.lastRun?.status === 'FAILED' && (
                <p className="mt-1 line-clamp-2 text-sm text-destructive">{job.lastRun.message}</p>
              )}
            </div>
          </button>
        </CollapsibleTrigger>
        <div className="flex items-center justify-between gap-4 text-sm sm:justify-end">
          <div className="text-muted-foreground sm:text-right">
            <div>
              Lần cuối:{' '}
              <span className="text-foreground" title={formatDateTime(job.lastRun?.startedAt)}>
                {job.lastRun ? fromNow(job.lastRun.startedAt) : 'chưa chạy'}
              </span>
            </div>
            <div>
              Kế tiếp:{' '}
              <span className="text-foreground" title={formatDateTime(job.nextRunAt)}>
                {job.nextRunAt ? fromNow(job.nextRunAt) : '—'}
              </span>
            </div>
          </div>
          <Button variant="outline" size="sm" disabled={busy} onClick={() => run.mutate(job.name)}>
            {busy ? <Loader2Icon className="animate-spin" /> : <PlayIcon />}
            Chạy ngay
          </Button>
        </div>
      </div>
      <CollapsibleContent className="border-t px-3 pb-2">
        {open && <Runs name={job.name} />}
      </CollapsibleContent>
    </Collapsible>
  )
}

export function JobsList() {
  const { data, error, isLoading, refetch } = useQuery(jobsQuery)
  if (error) return <ErrorState error={error} onRetry={() => void refetch()} />
  if (isLoading || !data) return <SkeletonRows rows={6} className="[&>*]:h-20" />
  if (data.length === 0) return <EmptyState title="Không có job nào" />
  const failed = data.filter((j) => j.lastRun?.status === 'FAILED')
  return (
    <div className="space-y-2">
      {failed.length > 0 && (
        <p className="text-sm text-destructive">
          {failed.length} job lỗi ở lần chạy gần nhất: {failed.map((j) => j.name).join(', ')}
        </p>
      )}
      {data.map((j) => (
        <JobRow key={j.name} job={j} />
      ))}
    </div>
  )
}
