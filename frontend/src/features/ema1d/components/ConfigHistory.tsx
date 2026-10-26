import { useQuery } from '@tanstack/react-query'

import { EmptyState, SkeletonRows } from '@/shared/components/states'
import { formatDateTime } from '@/shared/lib/date'

import { configHistoryQuery } from '../api'

function flatten(o: Record<string, unknown>, prefix = ''): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [k, v] of Object.entries(o)) {
    if (v && typeof v === 'object')
      Object.assign(out, flatten(v as Record<string, unknown>, `${prefix}${k}.`))
    else out[`${prefix}${k}`] = String(v)
  }
  return out
}

export function ConfigHistory() {
  const { data, isLoading } = useQuery(configHistoryQuery)
  if (isLoading || !data) return <SkeletonRows rows={3} />
  if (data.length === 0) return <EmptyState title="Chưa có thay đổi nào" />
  return (
    <ul className="space-y-3">
      {data.map((c) => {
        const before = flatten(c.before)
        const after = flatten(c.after)
        const keys = Object.keys(after).filter((k) => before[k] !== after[k])
        return (
          <li key={c.id} className="rounded-lg border p-3 text-sm">
            <p className="mb-1 text-muted-foreground">
              {formatDateTime(c.createdAt)} · {c.userEmail || 'hệ thống'}
            </p>
            {keys.map((k) => (
              <p key={k} className="num">
                <span className="text-muted-foreground">{k}:</span> {before[k] ?? '—'} →{' '}
                <span className="font-medium">{after[k]}</span>
              </p>
            ))}
          </li>
        )
      })}
    </ul>
  )
}
