import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2Icon } from 'lucide-react'
import { useForm, useWatch } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

import { ApiError, errorMessage } from '@/shared/api/errors'
import { formatMoney, formatPct } from '@/shared/lib/format'
import { Button } from '@/shared/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/shared/ui/dialog'
import { Field, FieldDescription, FieldError, FieldLabel } from '@/shared/ui/field'
import { NumberInput } from '@/shared/components/number-input'

import { type DcaSetting, type DcaTrade, useSaveTrade } from '../api'

const num = z.string().regex(/^\d+(\.\d+)?$/, 'Invalid number')
const schema = z
  .object({ buyPrice: num, sellPrice: z.string().regex(/^(\d+(\.\d+)?)?$/, 'Invalid number'), amount: num })
  .refine((v) => !v.sellPrice || Number(v.sellPrice) === 0 || Number(v.sellPrice) > Number(v.buyPrice), {
    message: 'Sell price must be above the buy price (or empty to hold)',
    path: ['sellPrice'],
  })
type Form = z.infer<typeof schema>

function distance(price: string, current?: string) {
  if (!current || !price || Number(price) <= 0) return ''
  return formatPct(((Number(price) - Number(current)) / Number(current)) * 100) + ' vs current price'
}

/** Adds or edits a rung. A held rung can only change its sell price. */
export function RungDialog({
  setting,
  rung,
  open,
  onOpenChange,
}: {
  setting: DcaSetting
  rung?: DcaTrade
  open: boolean
  onOpenChange: (o: boolean) => void
}) {
  const held = rung?.status === 'OPEN'
  const save = useSaveTrade(setting.id)
  const form = useForm<Form>({
    resolver: zodResolver(schema),
    values: {
      buyPrice: rung?.buyPrice ?? '',
      sellPrice: rung && rung.sellPrice !== '0' ? rung.sellPrice : '',
      amount: rung?.amount ?? '',
    },
  })
  const [buy, sell, amount] = useWatch({ control: form.control, name: ['buyPrice', 'sellPrice', 'amount'] })
  const notional = Number(buy) * Number(amount)

  const submit = form.handleSubmit(async (v) => {
    const body = held
      ? { sellPrice: v.sellPrice || '0' }
      : { buyPrice: v.buyPrice, sellPrice: v.sellPrice || '0', amount: v.amount }
    try {
      await save.mutateAsync({ id: rung?.id, body })
      toast.success(rung ? 'Rung saved' : 'Rung added')
      onOpenChange(false)
    } catch (err) {
      if (err instanceof ApiError && err.status === 400) {
        const d = err.details ?? {}
        for (const k of ['buyPrice', 'sellPrice', 'amount'] as const) {
          if (typeof d[k] === 'string') form.setError(k, { message: d[k] })
        }
        if (!Object.keys(d).length) form.setError('root', { message: err.message })
      } else toast.error(errorMessage(err))
    }
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={(e) => void submit(e)} className="space-y-4">
          <DialogHeader>
            <DialogTitle>{rung ? `Edit rung #${rung.id}` : 'Add rung'}</DialogTitle>
            <DialogDescription>
              {setting.pairName} · current price {setting.currentPrice ?? '—'}
              {setting.stepSize && ` · quantity step ${setting.stepSize}`}
            </DialogDescription>
          </DialogHeader>
          <Field data-invalid={!!form.formState.errors.buyPrice}>
            <FieldLabel htmlFor="buy">Buy when price ≤</FieldLabel>
            <NumberInput id="buy" disabled={held} {...form.register('buyPrice')} />
            <FieldDescription>{distance(buy, setting.currentPrice)}</FieldDescription>
            <FieldError>{form.formState.errors.buyPrice?.message}</FieldError>
          </Field>
          <Field data-invalid={!!form.formState.errors.amount}>
            <FieldLabel htmlFor="amount">Quantity ({setting.pairName.replace(/USDT$/, '')})</FieldLabel>
            <NumberInput id="amount" disabled={held} {...form.register('amount')} />
            <FieldDescription>
              {notional > 0 ? `≈ ${formatMoney(notional)} USDT` : ''}
              {setting.minNotional && ` · minimum ${setting.minNotional} USDT`}
            </FieldDescription>
            <FieldError>{form.formState.errors.amount?.message}</FieldError>
          </Field>
          <Field data-invalid={!!form.formState.errors.sellPrice}>
            <FieldLabel htmlFor="sell">Sell when price ≥</FieldLabel>
            <NumberInput
              id="sell"
              placeholder="empty: hold, never sell automatically"
              {...form.register('sellPrice')}
            />
            <FieldDescription>
              {sell && Number(buy) > 0
                ? `${formatPct(((Number(sell) - Number(buy)) / Number(buy)) * 100)} vs buy price`
                : ''}
            </FieldDescription>
            <FieldError>{form.formState.errors.sellPrice?.message}</FieldError>
          </Field>
          {form.formState.errors.root && (
            <p className="text-sm text-destructive">{form.formState.errors.root.message}</p>
          )}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={form.formState.isSubmitting}>
              {form.formState.isSubmitting && <Loader2Icon className="animate-spin" />}
              {rung ? 'Save' : 'Add rung'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
