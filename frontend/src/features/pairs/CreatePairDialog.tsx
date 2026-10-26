import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2Icon, PlusIcon } from 'lucide-react'
import { useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

import { ApiError, errorMessage } from '@/shared/api/errors'
import { Button } from '@/shared/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/shared/ui/dialog'
import { Field, FieldDescription, FieldError, FieldLabel } from '@/shared/ui/field'
import { Input } from '@/shared/ui/input'
import { ToggleGroup, ToggleGroupItem } from '@/shared/ui/toggle-group'

import { type Pair, useCreatePair } from './api'

const schema = z.object({
  name: z
    .string()
    .trim()
    .toUpperCase()
    .regex(/^[A-Z0-9]{2,20}$/, 'Chỉ gồm chữ và số, vd SUNUSDT'),
  account: z.enum(['CROSS', 'ISOLATED', 'SPOT']),
})
type Form = z.input<typeof schema>

export function CreatePairDialog({ onCreated }: { onCreated?: (p: Pair) => void }) {
  const [open, setOpen] = useState(false)
  const create = useCreatePair()
  const form = useForm<Form>({ resolver: zodResolver(schema), defaultValues: { name: '', account: 'CROSS' } })

  const submit = form.handleSubmit(async (raw) => {
    const v = schema.parse(raw)
    try {
      const p = await create.mutateAsync({
        name: v.name,
        isSpot: v.account === 'SPOT',
        isIsolated: v.account === 'ISOLATED',
      })
      toast.success(`Đã thêm ${p.name}`)
      setOpen(false)
      form.reset()
      onCreated?.(p)
    } catch (err) {
      // Binance rejected the symbol, or it already exists: show it on the field.
      if (err instanceof ApiError && err.status < 500) form.setError('name', { message: err.message })
      else toast.error(errorMessage(err))
    }
  })

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        setOpen(o)
        if (!o) form.reset()
      }}
    >
      <DialogTrigger asChild>
        <Button>
          <PlusIcon />
          Thêm pair
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={(e) => void submit(e)} className="space-y-4">
          <DialogHeader>
            <DialogTitle>Thêm pair</DialogTitle>
            <DialogDescription>
              Nhập symbol Binance. Coin, step size và min notional được lấy tự động từ Binance.
            </DialogDescription>
          </DialogHeader>
          <Field data-invalid={!!form.formState.errors.name}>
            <FieldLabel htmlFor="pair-name">Symbol</FieldLabel>
            <Input
              id="pair-name"
              placeholder="SUNUSDT"
              autoComplete="off"
              className="font-mono uppercase"
              aria-invalid={!!form.formState.errors.name}
              {...form.register('name')}
            />
            <FieldError>{form.formState.errors.name?.message}</FieldError>
          </Field>
          <Field>
            <FieldLabel>Giao dịch trên</FieldLabel>
            <Controller
              control={form.control}
              name="account"
              render={({ field }) => (
                <ToggleGroup
                  type="single"
                  variant="outline"
                  value={field.value}
                  onValueChange={(v) => {
                    if (v) field.onChange(v)
                  }}
                >
                  <ToggleGroupItem value="CROSS">Cross margin</ToggleGroupItem>
                  <ToggleGroupItem value="ISOLATED">Isolated</ToggleGroupItem>
                  <ToggleGroupItem value="SPOT">Spot</ToggleGroupItem>
                </ToggleGroup>
              )}
            />
            <FieldDescription>EMA1D cần cross margin. DCA dùng được cả spot.</FieldDescription>
          </Field>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setOpen(false)}>
              Huỷ
            </Button>
            <Button type="submit" disabled={form.formState.isSubmitting}>
              {form.formState.isSubmitting && <Loader2Icon className="animate-spin" />}
              Thêm
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
