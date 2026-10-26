import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'

import { ConfirmDialog } from '@/shared/components/confirm-dialog'
import { Label } from '@/shared/ui/label'
import { Switch } from '@/shared/ui/switch'

import { configQuery, type Ema1dConfig, useSaveConfig } from '../api'

type Key = 'enabled' | 'pauseOpen' | 'dailyCheckEnabled'

const LABELS: Record<Key, { label: string; on: string; off: string }> = {
  enabled: {
    label: 'Execute orders',
    on: 'The queue starts placing real orders on Binance (if TRADING_ENABLED is on).',
    off: 'The queue stops: todos are still created but not executed.',
  },
  pauseOpen: {
    label: 'Pause new entries',
    on: 'Sync, manual orders and the queue open no new positions. Closes still run.',
    off: 'Allow new entries again.',
  },
  dailyCheckEnabled: {
    label: 'Daily check 07:03',
    on: 'Every morning at 07:03 ICT: load candles, compute signals, sync and send the Discord report.',
    off: 'Stop running the daily check automatically.',
  },
}

/** The three strategy switches, each behind a confirmation that says what will happen. */
export function StrategySwitches() {
  const { data } = useQuery(configQuery)
  const save = useSaveConfig()
  const [pending, setPending] = useState<{ key: Key; value: boolean } | null>(null)
  if (!data) return null
  const cfg = data.config

  return (
    <div className="flex flex-wrap items-center gap-x-5 gap-y-2">
      {(Object.keys(LABELS) as Key[]).map((key) => (
        <div key={key} className="flex items-center gap-2">
          <Switch
            id={`sw-${key}`}
            checked={cfg[key]}
            onCheckedChange={(value) => setPending({ key, value })}
            disabled={save.isPending}
          />
          <Label htmlFor={`sw-${key}`} className="text-sm">
            {LABELS[key].label}
          </Label>
        </div>
      ))}
      {pending && (
        <ConfirmDialog
          open
          onOpenChange={(o) => {
            if (!o) setPending(null)
          }}
          title={`Turn ${pending.value ? 'on' : 'off'} "${LABELS[pending.key].label}"?`}
          description={pending.value ? LABELS[pending.key].on : LABELS[pending.key].off}
          destructive={pending.key === 'enabled' && pending.value}
          confirmLabel={pending.value ? 'Turn on' : 'Turn off'}
          onConfirm={async () => {
            const next: Ema1dConfig = { ...cfg, [pending.key]: pending.value }
            await save.mutateAsync({ version: data.version, config: next })
          }}
        />
      )}
    </div>
  )
}
