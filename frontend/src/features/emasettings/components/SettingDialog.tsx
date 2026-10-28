import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Loader2Icon } from 'lucide-react'
import { useState } from 'react'

import { pairsQuery } from '@/features/pairs/api'
import { Combobox } from '@/shared/components/combobox'
import { NumberInput } from '@/shared/components/number-input'
import { Button } from '@/shared/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/shared/ui/dialog'
import { Label } from '@/shared/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui/select'
import { Switch } from '@/shared/ui/switch'
import { Textarea } from '@/shared/ui/textarea'

import {
  CHECK_TYPE_LABEL,
  CHECK_TYPES,
  type EmaSetting,
  PERIODS,
  TRADE_TYPE_LABEL,
  TRADE_TYPES,
  useSaveSetting,
} from '../api'

type Form = {
  pairId: string
  budget: string
  tradeType: EmaSetting['tradeType']
  period: EmaSetting['period']
  checkType: EmaSetting['checkType']
  newTrade: boolean
  tpPercent: string
  slPercent: string
  note: string
}

const isNum = (v: string) => /^\d+(\.\d+)?$/.test(v)

function initial(s?: EmaSetting): Form {
  return {
    pairId: s ? String(s.pairId) : '',
    budget: s?.budget ?? '',
    tradeType: s?.tradeType ?? 'BOTH',
    period: s?.period ?? 'D1',
    checkType: s?.checkType ?? 'CHECK_AT_OPEN',
    newTrade: s ? s.nextTradeType === 'NEW_TRADE' : true,
    tpPercent: s?.tpPercent ?? '0',
    slPercent: s?.slPercent ?? '0',
    note: s?.note ?? '',
  }
}

/** Creates a campaign (no `setting`) or edits one. */
export function SettingDialog({
  open,
  onOpenChange,
  setting,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  setting?: EmaSetting
}) {
  const { data: pairs } = useQuery({ ...pairsQuery({ status: 'ACTIVE' }), enabled: open && !setting })
  const save = useSaveSetting()
  const navigate = useNavigate()
  const [form, setForm] = useState<Form>(() => initial(setting))
  const [prev, setPrev] = useState<{ open: boolean; setting?: EmaSetting }>({ open: false })

  // Reset the form each time the dialog opens.
  if (prev.open !== open || prev.setting !== setting) {
    setPrev({ open, setting })
    if (open) setForm(initial(setting))
  }
  const set = <K extends keyof Form>(k: K, v: Form[K]) => setForm((f) => ({ ...f, [k]: v }))

  const options = (pairs ?? [])
    .filter((p) => !p.isIsolated)
    .map((p) => ({ value: String(p.id), label: p.name, hint: p.isSpot ? 'Spot' : 'Cross' }))
  const valid =
    (setting || form.pairId) &&
    isNum(form.budget) &&
    Number(form.budget) > 0 &&
    isNum(form.tpPercent) &&
    isNum(form.slPercent)

  const submit = () =>
    save.mutate(
      {
        id: setting?.id,
        body: {
          pairId: setting ? undefined : Number(form.pairId),
          budget: form.budget,
          tradeType: form.tradeType,
          period: form.period,
          checkType: form.checkType,
          nextTradeType: form.newTrade ? 'NEW_TRADE' : 'NOT_NEW_TRADE',
          tpPercent: form.tpPercent,
          slPercent: form.slPercent,
          note: form.note,
        },
      },
      {
        onSuccess: (s) => {
          onOpenChange(false)
          if (!setting) void navigate({ to: '/ema-cross/campaigns/$id', params: { id: String(s.id) } })
        },
      },
    )

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{setting ? `Edit campaign · ${setting.pairName}` : 'New campaign'}</DialogTitle>
          <DialogDescription>
            Trades EMA14 crossing Smooth10 on one pair, one trade at a time sized by the budget.
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-4 sm:grid-cols-2">
          {!setting && (
            <div className="grid gap-2 sm:col-span-2">
              <Label htmlFor="es-pair">Pair</Label>
              <Combobox
                id="es-pair"
                options={options}
                value={form.pairId}
                onChange={(v) => set('pairId', v)}
                placeholder="Select a pair"
                searchPlaceholder="Search a coin"
              />
            </div>
          )}
          <div className="grid gap-2">
            <Label htmlFor="es-budget">Budget per trade (USDT)</Label>
            <NumberInput
              id="es-budget"
              value={form.budget}
              onChange={(e) => set('budget', e.target.value)}
              aria-invalid={!!form.budget && !isNum(form.budget)}
            />
          </div>
          <div className="grid gap-2">
            <Label>Sides</Label>
            <Select value={form.tradeType} onValueChange={(v) => set('tradeType', v as Form['tradeType'])}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {TRADE_TYPES.map((t) => (
                  <SelectItem key={t} value={t}>
                    {TRADE_TYPE_LABEL[t]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="grid gap-2">
            <Label>Timeframe</Label>
            <Select value={form.period} onValueChange={(v) => set('period', v as Form['period'])}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {PERIODS.map((p) => (
                  <SelectItem key={p} value={p}>
                    {p}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="grid gap-2">
            <Label>EMA check</Label>
            <Select value={form.checkType} onValueChange={(v) => set('checkType', v as Form['checkType'])}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {CHECK_TYPES.map((c) => (
                  <SelectItem key={c} value={c}>
                    {CHECK_TYPE_LABEL[c]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="grid gap-2">
            <Label htmlFor="es-tp">Campaign take profit (% of budget)</Label>
            <NumberInput
              id="es-tp"
              value={form.tpPercent}
              onChange={(e) => set('tpPercent', e.target.value)}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="es-sl">Campaign stop loss (% of budget)</Label>
            <NumberInput
              id="es-sl"
              value={form.slPercent}
              onChange={(e) => set('slPercent', e.target.value)}
            />
          </div>
          <div className="flex items-center gap-3 sm:col-span-2">
            <Switch id="es-next" checked={form.newTrade} onCheckedChange={(v) => set('newTrade', v)} />
            <Label htmlFor="es-next">
              {form.newTrade
                ? 'Open a new trade on the next cross after each close'
                : 'One trade only: no new trade after a close'}
            </Label>
          </div>
          <p className="text-xs text-muted-foreground sm:col-span-2">
            0 turns the campaign take profit / stop loss off. Each trade also gets the stop loss % of its
            timeframe (Settings).
          </p>
          <div className="grid gap-2 sm:col-span-2">
            <Label htmlFor="es-note">Note</Label>
            <Textarea id="es-note" rows={3} value={form.note} onChange={(e) => set('note', e.target.value)} />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button disabled={!valid || save.isPending} onClick={submit}>
            {save.isPending && <Loader2Icon className="animate-spin" />}
            {setting ? 'Save' : 'Create'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
