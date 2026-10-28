import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'

import { ConfirmDialog } from '@/shared/components/confirm-dialog'
import { NumberInput } from '@/shared/components/number-input'
import { ErrorState, SkeletonRows } from '@/shared/components/states'
import { Button } from '@/shared/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card'
import { Label } from '@/shared/ui/label'
import { Switch } from '@/shared/ui/switch'

import { type EmaSettingsConfig, emaSettingsConfigQuery, PERIODS, useSaveEmaSettingsConfig } from '../api'

type Params = EmaSettingsConfig['d1']
const FIELDS: { key: keyof Params; label: string }[] = [
  { key: 'openPercent', label: 'Enter: EMA14 past Smooth10 by (%)' },
  { key: 'closePercent', label: 'Exit: cross back by (%)' },
  { key: 'ignoreOverEmaPercent', label: 'No entry when price is this far from EMA14 (%)' },
  { key: 'stopLossPercent', label: 'Stop loss per trade, from the entry (%)' },
]
const isPercent = (v: string) => /^\d+(\.\d+)?$/.test(v) && Number(v) <= 100
const key = (p: (typeof PERIODS)[number]) => (p === 'D1' ? 'd1' : 'h4')

/** The Campaigns switch and thresholds, shown on the EMA Cross settings page. */
export function CampaignConfigCards() {
  const { data, error, refetch } = useQuery(emaSettingsConfigQuery)
  const save = useSaveEmaSettingsConfig()
  const [edit, setEdit] = useState<EmaSettingsConfig | null>(null)
  const [pending, setPending] = useState<EmaSettingsConfig | null>(null)

  if (error) return <ErrorState error={error} onRetry={() => void refetch()} />
  if (!data) return <SkeletonRows rows={3} />
  const c = data.config
  const values = edit ?? c
  const valid = PERIODS.every((p) => FIELDS.every((f) => isPercent(values[key(p)][f.key])))
  const changed = PERIODS.some((p) => FIELDS.some((f) => values[key(p)][f.key] !== c[key(p)][f.key]))
  const setField = (p: (typeof PERIODS)[number], f: keyof Params, v: string) =>
    setEdit({ ...values, [key(p)]: { ...values[key(p)], [f]: v } })

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Campaigns scan</CardTitle>
          <CardDescription>
            Every 5 minutes: stop loss, target and cross back of held trades, new trades on the cross,
            campaign take profit / stop loss. Off: nothing happens on its own (buttons still place orders).
          </CardDescription>
        </CardHeader>
        <CardContent className="flex items-center gap-3">
          <Switch
            id="es-enabled"
            checked={c.enabled}
            onCheckedChange={(v) => setPending({ ...c, enabled: v })}
          />
          <Label htmlFor="es-enabled">{c.enabled ? 'On' : 'Off'}</Label>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Campaign thresholds</CardTitle>
          <CardDescription>Per timeframe, shared by every campaign of that timeframe.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            {PERIODS.map((p) => (
              <div key={p} className="space-y-3">
                <p className="text-sm font-medium">{p}</p>
                {FIELDS.map((f) => (
                  <div key={f.key} className="grid gap-1.5">
                    <Label htmlFor={`es-${p}-${f.key}`} className="text-xs text-muted-foreground">
                      {f.label}
                    </Label>
                    <NumberInput
                      id={`es-${p}-${f.key}`}
                      value={values[key(p)][f.key]}
                      onChange={(e) => setField(p, f.key, e.target.value)}
                      aria-invalid={!isPercent(values[key(p)][f.key])}
                    />
                  </div>
                ))}
              </div>
            ))}
          </div>
          <Button disabled={!valid || !changed} onClick={() => setPending({ ...values, enabled: c.enabled })}>
            Save thresholds
          </Button>
        </CardContent>
      </Card>
      {pending && (
        <ConfirmDialog
          open
          onOpenChange={(o) => !o && setPending(null)}
          title="Save campaign settings?"
          description={`Scan: ${pending.enabled ? 'on' : 'off'} · D1 enter ${pending.d1.openPercent}% / exit ${pending.d1.closePercent}% / SL ${pending.d1.stopLossPercent}% · H4 enter ${pending.h4.openPercent}% / exit ${pending.h4.closePercent}% / SL ${pending.h4.stopLossPercent}%`}
          onConfirm={async () => {
            await save.mutateAsync({ version: data.version, config: pending })
            setEdit(null)
          }}
        />
      )}
    </>
  )
}
