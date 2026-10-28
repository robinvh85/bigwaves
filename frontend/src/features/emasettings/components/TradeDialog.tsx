import { Loader2Icon } from 'lucide-react'
import { useState } from 'react'

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
import { ToggleGroup, ToggleGroupItem } from '@/shared/ui/toggle-group'

import { type EmaSetting, type EmaSettingTrade, useCreateTrade, useUpdateTrade } from '../api'

type Form = {
  side: EmaSettingTrade['side']
  status: 'OPENING' | 'OPEN'
  amount: string
  entryPrice: string
  openPrice: string
  targetPrice: string
  slPrice: string
}

const okNum = (v: string) => v === '' || /^\d+(\.\d+)?$/.test(v)
const orZero = (v: string) => (v === '' ? '0' : v)

function initial(t?: EmaSettingTrade): Form {
  return {
    side: t?.side ?? 'LONG',
    status: t?.status === 'OPEN' ? 'OPEN' : 'OPENING',
    amount: t && t.amount !== '0' ? t.amount : '',
    entryPrice: t?.entryPrice ?? '',
    openPrice: t?.openPrice ?? '',
    targetPrice: t?.targetPrice ?? '',
    slPrice: t?.slPrice ?? '',
  }
}

/**
 * Adds a trade by hand (no `trade`) or edits one. A waiting trade can change everything; a held one
 * only its amount, target and stop loss. Adding a held trade records a position already open on
 * Binance: no order is placed.
 */
export function TradeDialog({
  open,
  onOpenChange,
  setting,
  trade,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  setting: EmaSetting
  trade?: EmaSettingTrade
}) {
  const create = useCreateTrade(setting.id)
  const update = useUpdateTrade()
  const [form, setForm] = useState<Form>(() => initial(trade))
  const [prev, setPrev] = useState<{ open: boolean; trade?: EmaSettingTrade }>({ open: false })
  if (prev.open !== open || prev.trade !== trade) {
    setPrev({ open, trade })
    if (open) setForm(initial(trade))
  }
  const set = <K extends keyof Form>(k: K, v: Form[K]) => setForm((f) => ({ ...f, [k]: v }))

  const held = trade ? trade.status === 'OPEN' : form.status === 'OPEN'
  const coin = setting.pairName.replace(/USDT$/, '')
  const nums: (keyof Form)[] = ['amount', 'entryPrice', 'openPrice', 'targetPrice', 'slPrice']
  const valid =
    nums.every((k) => okNum(form[k])) &&
    (!held || trade || (form.amount !== '' && form.openPrice !== '')) &&
    (!trade || form.amount !== '')
  const pending = create.isPending || update.isPending

  const submit = () => {
    const done = { onSuccess: () => onOpenChange(false) }
    if (!trade) {
      create.mutate(
        {
          side: form.side,
          status: form.status,
          amount: form.amount || undefined,
          entryPrice: held ? undefined : form.entryPrice || undefined,
          openPrice: held ? form.openPrice : undefined,
          targetPrice: form.targetPrice || undefined,
          slPrice: form.slPrice || undefined,
        },
        done,
      )
      return
    }
    update.mutate(
      {
        id: trade.id,
        body: {
          amount: form.amount,
          targetPrice: orZero(form.targetPrice),
          slPrice: orZero(form.slPrice),
          ...(held ? {} : { side: form.side, entryPrice: orZero(form.entryPrice) }),
        },
      },
      done,
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{trade ? `Edit trade #${trade.id}` : 'Add trade'}</DialogTitle>
          <DialogDescription>
            {setting.pairName} · current price {setting.currentPrice ?? '—'}
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="grid gap-2">
            <Label>Side</Label>
            <Select
              value={form.side}
              onValueChange={(v) => set('side', v as Form['side'])}
              disabled={held && !!trade}
            >
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="LONG">LONG</SelectItem>
                <SelectItem value="SHORT">SHORT</SelectItem>
              </SelectContent>
            </Select>
          </div>
          {!trade && (
            <div className="grid gap-2">
              <Label>State</Label>
              <ToggleGroup
                type="single"
                variant="outline"
                value={form.status}
                onValueChange={(v) => v && set('status', v as Form['status'])}
              >
                <ToggleGroupItem value="OPENING">Waiting</ToggleGroupItem>
                <ToggleGroupItem value="OPEN">Already held</ToggleGroupItem>
              </ToggleGroup>
            </div>
          )}
          <div className="grid gap-2 sm:col-span-2">
            <Label htmlFor="es-amount">Amount ({coin})</Label>
            <NumberInput
              id="es-amount"
              value={form.amount}
              placeholder={held ? '' : `empty: budget ${setting.budget} USDT at the open price`}
              onChange={(e) => set('amount', e.target.value)}
            />
          </div>
          {held ? (
            !trade && (
              <div className="grid gap-2 sm:col-span-2">
                <Label htmlFor="es-open">Entry fill price</Label>
                <NumberInput
                  id="es-open"
                  value={form.openPrice}
                  onChange={(e) => set('openPrice', e.target.value)}
                />
              </div>
            )
          ) : (
            <div className="grid gap-2 sm:col-span-2">
              <Label htmlFor="es-entry">Enter when the price passes</Label>
              <NumberInput
                id="es-entry"
                value={form.entryPrice}
                placeholder="empty: on the EMA cross"
                onChange={(e) => set('entryPrice', e.target.value)}
              />
              <p className="text-xs text-muted-foreground">
                A long enters below it, a short above it (like a limit order).
              </p>
            </div>
          )}
          <div className="grid gap-2">
            <Label htmlFor="es-target">Target price</Label>
            <NumberInput
              id="es-target"
              value={form.targetPrice}
              placeholder="none"
              onChange={(e) => set('targetPrice', e.target.value)}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="es-sl">Stop loss price</Label>
            <NumberInput
              id="es-sl"
              value={form.slPrice}
              placeholder={held && trade ? 'none' : 'empty: % of the timeframe'}
              onChange={(e) => set('slPrice', e.target.value)}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button disabled={!valid || pending} onClick={submit}>
            {pending && <Loader2Icon className="animate-spin" />}
            {trade ? 'Save' : 'Add'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
