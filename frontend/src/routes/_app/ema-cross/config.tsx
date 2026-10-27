import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'

import {
  type EmaCrossConfig,
  emaCrossConfigQuery,
  emaCrossHistoryQuery,
  TIMEFRAMES,
  useSaveEmaCrossConfig,
  watchKey,
} from '@/features/emacross/api'
import { ConfirmDialog } from '@/shared/components/confirm-dialog'
import { NumberInput } from '@/shared/components/number-input'
import { ErrorState, PageHeader, SkeletonRows } from '@/shared/components/states'
import { formatDateTime } from '@/shared/lib/date'
import { Button } from '@/shared/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card'
import { Label } from '@/shared/ui/label'
import { Switch } from '@/shared/ui/switch'

export const Route = createFileRoute('/_app/ema-cross/config')({
  staticData: { title: 'Settings' },
  component: EmaCrossConfigPage,
})

type Thresholds = EmaCrossConfig['thresholdPercent']

const isPercent = (v: string) => /^\d+(\.\d+)?$/.test(v) && Number(v) <= 100

function EmaCrossConfigPage() {
  const { data, error, refetch } = useQuery(emaCrossConfigQuery)
  const { data: history } = useQuery(emaCrossHistoryQuery)
  const save = useSaveEmaCrossConfig()
  const [edit, setEdit] = useState<Thresholds | null>(null)
  const [pending, setPending] = useState<EmaCrossConfig | null>(null)

  if (error) return <ErrorState error={error} onRetry={() => void refetch()} />
  if (!data) return <SkeletonRows rows={4} />
  const c = data.config
  const values = edit ?? c.thresholdPercent
  const valid = TIMEFRAMES.every((tf) => isPercent(values[watchKey(tf)]))
  const changed = TIMEFRAMES.some((tf) => values[watchKey(tf)] !== c.thresholdPercent[watchKey(tf)])

  return (
    <>
      <PageHeader title="EMA Cross settings" />
      <div className="max-w-2xl space-y-4">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Alerts</CardTitle>
            <CardDescription>Off: the checks skip and nothing is sent.</CardDescription>
          </CardHeader>
          <CardContent className="flex items-center gap-3">
            <Switch
              id="emacross-enabled"
              checked={c.enabled}
              onCheckedChange={(v) => setPending({ ...c, enabled: v })}
            />
            <Label htmlFor="emacross-enabled">{c.enabled ? 'On' : 'Off'}</Label>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Cross threshold</CardTitle>
            <CardDescription>
              How far EMA14 must pass Smooth10 the other way, in % of the lower line, to count as a cross.
              Higher values send fewer, later alerts.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-3 sm:flex-row sm:items-end">
            {TIMEFRAMES.map((tf) => (
              <div key={tf} className="grid flex-1 gap-2">
                <Label htmlFor={`emacross-${tf}`}>{tf} (%)</Label>
                <NumberInput
                  id={`emacross-${tf}`}
                  value={values[watchKey(tf)]}
                  onChange={(e) => setEdit({ ...values, [watchKey(tf)]: e.target.value })}
                  aria-invalid={!isPercent(values[watchKey(tf)])}
                />
              </div>
            ))}
            <Button
              disabled={!valid || !changed}
              onClick={() => setPending({ ...c, thresholdPercent: values })}
            >
              Save
            </Button>
          </CardContent>
        </Card>
        {history && history.length > 0 && (
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Change history</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              {history.map((h) => (
                <p key={h.id}>
                  <span className="text-muted-foreground">
                    {formatDateTime(h.createdAt)} · {h.userEmail}:
                  </span>{' '}
                  {JSON.stringify(h.before)} → {JSON.stringify(h.after)}
                </p>
              ))}
            </CardContent>
          </Card>
        )}
      </div>
      {pending && (
        <ConfirmDialog
          open
          onOpenChange={(o) => !o && setPending(null)}
          title="Save EMA Cross settings?"
          description={`Alerts: ${pending.enabled ? 'on' : 'off'} · threshold H1 ${pending.thresholdPercent.h1}% · H4 ${pending.thresholdPercent.h4}% · D1 ${pending.thresholdPercent.d1}%`}
          onConfirm={async () => {
            await save.mutateAsync({ version: data.version, config: pending })
            setEdit(null)
          }}
        />
      )}
    </>
  )
}
