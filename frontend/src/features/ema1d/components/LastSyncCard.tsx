import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card'
import { formatDateTime, fromNow } from '@/shared/lib/date'

import type { Ema1dStatus } from '../api'

type Skipped = NonNullable<Ema1dStatus['lastSync']>['skipped']

/** Groups skipped coins by reason ("entry_too_old (before …)" → "entry_too_old"), like the report. */
function groupSkipped(items: Skipped): [string, string[]][] {
  const groups = new Map<string, string[]>()
  for (const k of items) {
    const reason = k.reason.split(' (')[0] ?? k.reason
    groups.set(reason, [...(groups.get(reason) ?? []), k.coin])
  }
  return [...groups.entries()]
}

export function LastSyncCard({ status }: { status: Ema1dStatus }) {
  const s = status.lastSync
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Sync gần nhất</CardTitle>
        <CardDescription>
          {s ? (
            <span title={formatDateTime(s.at)}>
              {s.runId} · nến {s.date} · {fromNow(s.at)}
            </span>
          ) : (
            'Chưa có sync nào'
          )}
        </CardDescription>
      </CardHeader>
      {s && (
        <CardContent className="space-y-3 text-sm">
          <p>
            <span className="text-muted-foreground">Todo mới:</span>{' '}
            <span className="num">{s.createdTodos}</span>
          </p>
          {s.skipped.length > 0 && (
            <div>
              <p className="text-muted-foreground">Bỏ qua ({s.skipped.length})</p>
              <ul className="mt-1 space-y-1">
                {groupSkipped(s.skipped).map(([reason, coins]) => (
                  <li key={reason}>
                    <span className="text-muted-foreground">{reason}:</span>{' '}
                    <span className="font-mono">{coins.join(', ')}</span>
                  </li>
                ))}
              </ul>
            </div>
          )}
          {s.warnings.length > 0 && (
            <div>
              <p className="text-muted-foreground">Cảnh báo ({s.warnings.length})</p>
              <ul className="mt-1 list-inside list-disc space-y-0.5 text-warning">
                {s.warnings.map((w, i) => (
                  <li key={i}>{w}</li>
                ))}
              </ul>
            </div>
          )}
        </CardContent>
      )}
    </Card>
  )
}
