import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'

import { dcaConfigQuery, dcaHistoryQuery, useSaveDcaConfig } from '@/features/dca/api'
import { ConfirmDialog } from '@/shared/components/confirm-dialog'
import { ErrorState, PageHeader, SkeletonRows } from '@/shared/components/states'
import { formatDateTime } from '@/shared/lib/date'
import { Button } from '@/shared/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card'
import { NumberInput } from '@/shared/components/number-input'
import { Label } from '@/shared/ui/label'
import { Switch } from '@/shared/ui/switch'

export const Route = createFileRoute('/_app/dca/config')({
  staticData: { title: 'Settings' },
  component: DcaConfigPage,
})

function DcaConfigPage() {
  const { data, error, refetch } = useQuery(dcaConfigQuery)
  const { data: history } = useQuery(dcaHistoryQuery)
  const save = useSaveDcaConfig()
  const [cap, setCap] = useState<string | null>(null)
  const [pending, setPending] = useState<{ enabled: boolean; maxCapitalUsdt: string } | null>(null)

  if (error) return <ErrorState error={error} onRetry={() => void refetch()} />
  if (!data) return <SkeletonRows rows={4} />
  const c = data.config
  const capValue = cap ?? c.maxCapitalUsdt
  const capValid = /^\d+(\.\d+)?$/.test(capValue)

  return (
    <>
      <PageHeader title="DCA settings" />
      <div className="max-w-2xl space-y-4">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Automatic trading</CardTitle>
            <CardDescription>Off: ladders are kept but no orders are filled.</CardDescription>
          </CardHeader>
          <CardContent className="flex items-center gap-3">
            <Switch
              id="dca-enabled"
              checked={c.enabled}
              onCheckedChange={(v) => setPending({ ...c, enabled: v })}
            />
            <Label htmlFor="dca-enabled">{c.enabled ? 'On' : 'Off'}</Label>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Capital cap</CardTitle>
            <CardDescription>
              Total USDT in bought rungs across all ladders. A buy that would exceed the cap is refused. 0 =
              no cap.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex items-end gap-2">
            <div className="grid flex-1 gap-2">
              <Label htmlFor="dca-cap">USDT</Label>
              <NumberInput
                id="dca-cap"
                value={capValue}
                onChange={(e) => setCap(e.target.value)}
                aria-invalid={!capValid}
              />
            </div>
            <Button
              disabled={!capValid || capValue === c.maxCapitalUsdt}
              onClick={() => setPending({ ...c, maxCapitalUsdt: capValue })}
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
          title="Save DCA settings?"
          description={`Automatic trading: ${pending.enabled ? 'on' : 'off'} · capital cap: ${pending.maxCapitalUsdt === '0' ? 'none' : `${pending.maxCapitalUsdt} USDT`}`}
          onConfirm={async () => {
            await save.mutateAsync({ version: data.version, config: pending })
            setCap(null)
          }}
        />
      )}
    </>
  )
}
