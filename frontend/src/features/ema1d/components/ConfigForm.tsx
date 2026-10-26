import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { Loader2Icon } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { z } from 'zod'

import { ConfirmDialog } from '@/shared/components/confirm-dialog'
import { ErrorState, SkeletonRows } from '@/shared/components/states'
import { formatMoney } from '@/shared/lib/format'
import { Button } from '@/shared/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card'
import { Field, FieldDescription, FieldError, FieldLabel } from '@/shared/ui/field'
import { NumberInput } from '@/shared/components/number-input'

import { configQuery, type Ema1dConfig, type Ema1dConfigEnvelope, statusQuery, useSaveConfig } from '../api'

const dec = z.string().regex(/^\d+(\.\d+)?$/, 'Invalid number')
const pct = dec.refine((v) => Number(v) <= 100, 'At most 100%')
const schema = z
  .object({
    budgetUsdt: dec,
    maxEntryAgeDays: z.coerce.number().int().min(0).max(30),
    coreLong: pct,
    coreShort: pct,
    sensitiveLong: pct,
    sensitiveShort: pct,
    marginLevelWarn: dec,
    marginLevelAlert: dec,
  })
  .refine((v) => Number(v.marginLevelAlert) <= Number(v.marginLevelWarn), {
    message: 'Alert level must be ≤ warning level',
    path: ['marginLevelAlert'],
  })
type FormIn = z.input<typeof schema>
type Form = z.output<typeof schema>

function toForm(c: Ema1dConfig): FormIn {
  return {
    budgetUsdt: c.budgetUsdt,
    maxEntryAgeDays: c.maxEntryAgeDays,
    coreLong: c.sizes.coreLong,
    coreShort: c.sizes.coreShort,
    sensitiveLong: c.sizes.sensitiveLong,
    sensitiveShort: c.sizes.sensitiveShort,
    marginLevelWarn: c.marginLevelWarn,
    marginLevelAlert: c.marginLevelAlert,
  }
}

function toConfig(base: Ema1dConfig, f: Form): Ema1dConfig {
  return {
    ...base,
    budgetUsdt: f.budgetUsdt,
    maxEntryAgeDays: f.maxEntryAgeDays,
    sizes: {
      coreLong: f.coreLong,
      coreShort: f.coreShort,
      sensitiveLong: f.sensitiveLong,
      sensitiveShort: f.sensitiveShort,
    },
    marginLevelWarn: f.marginLevelWarn,
    marginLevelAlert: f.marginLevelAlert,
  }
}

const LABELS: Record<keyof Form, string> = {
  budgetUsdt: 'Budget',
  maxEntryAgeDays: 'Max signal age',
  coreLong: 'Core · LONG',
  coreShort: 'Core · SHORT',
  sensitiveLong: 'Sensitive · LONG',
  sensitiveShort: 'Sensitive · SHORT',
  marginLevelWarn: 'Margin level warning',
  marginLevelAlert: 'Margin level alert',
}

export function ConfigForm() {
  const { data, error, refetch } = useQuery(configQuery)
  if (error) return <ErrorState error={error} onRetry={() => void refetch()} />
  if (!data) return <SkeletonRows rows={8} />
  return <Editor key={data.version} env={data} />
}

function Editor({ env }: { env: Ema1dConfigEnvelope }) {
  const { data: status } = useQuery(statusQuery)
  const save = useSaveConfig()
  const form = useForm<FormIn, unknown, Form>({
    resolver: zodResolver(schema),
    defaultValues: toForm(env.config),
  })
  const [review, setReview] = useState<Form | null>(null)
  const equity = Number(status?.capital.equityUsdt ?? env.config.budgetUsdt)
  const realized = equity - Number(env.config.budgetUsdt)

  const field = (name: keyof Form, opts: { unit?: string; hint?: string; suggested?: string }) => (
    <Field data-invalid={!!form.formState.errors[name]}>
      <FieldLabel htmlFor={name}>{LABELS[name]}</FieldLabel>
      <div className="relative">
        <NumberInput id={name} className="pr-14" {...form.register(name)} />
        {opts.unit && (
          <span className="absolute top-1/2 right-3 -translate-y-1/2 text-sm text-muted-foreground">
            {opts.unit}
          </span>
        )}
      </div>
      {opts.suggested && (
        <FieldDescription>
          The backtest uses {opts.suggested}%. Lower = trial run: orders below the Binance minimum are raised
          to the minimum.
        </FieldDescription>
      )}
      {opts.hint && <FieldDescription>{opts.hint}</FieldDescription>}
      <FieldError>{form.formState.errors[name]?.message}</FieldError>
    </Field>
  )

  const changes = review
    ? (Object.keys(LABELS) as (keyof Form)[]).filter(
        (k) => String(review[k]) !== String(toForm(env.config)[k]),
      )
    : []
  const notional = (budget: string, p: string) => ((Number(budget) + realized) * Number(p)) / 100

  return (
    <form onSubmit={(e) => void form.handleSubmit((v) => setReview(v))(e)} className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Capital & sizing</CardTitle>
          <CardDescription>
            Size = % × strategy capital (budget + realized PnL, now {formatMoney(equity)} USDT). The actual
            balance is not used.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 sm:grid-cols-2">
          {field('budgetUsdt', { unit: 'USDT' })}
          <div />
          {field('coreLong', { unit: '%', suggested: env.suggestedSizes.coreLong })}
          {field('coreShort', { unit: '%', suggested: env.suggestedSizes.coreShort })}
          {field('sensitiveLong', { unit: '%', suggested: env.suggestedSizes.sensitiveLong })}
          {field('sensitiveShort', { unit: '%', suggested: env.suggestedSizes.sensitiveShort })}
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Entries & alerts</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-4 sm:grid-cols-2">
          {field('maxEntryAgeDays', {
            unit: 'days',
            hint: 'Entry signals older than this are not opened (exit signals always run).',
          })}
          <div />
          {field('marginLevelWarn', {
            hint: 'The monitor warns on Discord at most once an hour while margin level is below this.',
          })}
          {field('marginLevelAlert', { hint: 'Alert: short squeeze / liquidation risk.' })}
        </CardContent>
      </Card>
      <div className="flex justify-end gap-2">
        <Button
          type="button"
          variant="outline"
          disabled={!form.formState.isDirty}
          onClick={() => form.reset(toForm(env.config))}
        >
          Undo
        </Button>
        <Button type="submit" disabled={!form.formState.isDirty}>
          Review & save
        </Button>
      </div>

      {review && (
        <ConfirmDialog
          open
          onOpenChange={(o) => !o && setReview(null)}
          title="Save EMA1D settings?"
          description="Applies from the next opened trade. Held positions do not change."
          confirmLabel="Save"
          onConfirm={() => save.mutateAsync({ version: env.version, config: toConfig(env.config, review) })}
        >
          <ul className="space-y-1 text-sm">
            {changes.map((k) => {
              const before = String(toForm(env.config)[k])
              const after = String(review[k])
              const isSize = k.endsWith('Long') || k.endsWith('Short')
              return (
                <li key={k} className="flex flex-wrap justify-between gap-2">
                  <span className="text-muted-foreground">{LABELS[k]}</span>
                  <span className="num">
                    {before} → <span className="font-semibold">{after}</span>
                    {isSize && (
                      <span className="ml-2 text-xs text-muted-foreground">
                        ≈ {formatMoney(notional(review.budgetUsdt, before))} →{' '}
                        {formatMoney(notional(review.budgetUsdt, after))} USDT/trade
                      </span>
                    )}
                  </span>
                </li>
              )
            })}
          </ul>
          {save.isPending && <Loader2Icon className="mx-auto animate-spin" />}
        </ConfirmDialog>
      )}
    </form>
  )
}
