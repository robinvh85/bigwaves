import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { Loader2Icon, PlusIcon } from 'lucide-react'
import { useState } from 'react'
import { Controller, useForm, useWatch } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

import { ApiError, errorMessage } from '@/shared/api/errors'
import { dayjs } from '@/shared/lib/date'
import { Button } from '@/shared/ui/button'
import { Checkbox } from '@/shared/ui/checkbox'
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
import { Label } from '@/shared/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui/select'
import { ToggleGroup, ToggleGroupItem } from '@/shared/ui/toggle-group'

import { pairsQuery, useCreateTodo } from '../api'

const schema = z
  .object({
    action: z.enum(['OPEN', 'CLOSE']),
    coin: z.string().min(1, 'Chọn coin'),
    side: z.enum(['LONG', 'SHORT']).optional(),
    signalDate: z.string().optional(),
    refPrice: z
      .string()
      .regex(/^(\d+(\.\d+)?)?$/, 'Giá không hợp lệ')
      .optional(),
    rule: z.enum(['XO', 'DIV4H']).optional(),
    force: z.boolean(),
  })
  .refine((v) => v.action === 'CLOSE' || (v.side && v.signalDate), {
    message: 'OPEN cần chiều và ngày tín hiệu',
    path: ['signalDate'],
  })
type Form = z.infer<typeof schema>

/** One OPEN or CLOSE by hand, for when the sync cannot fix things. Same checks as the sync. */
export function ManualTodoDialog({ coin }: { coin?: string }) {
  const [open, setOpen] = useState(false)
  const { data: pairs } = useQuery(pairsQuery)
  const create = useCreateTodo()
  const form = useForm<Form>({
    resolver: zodResolver(schema),
    defaultValues: {
      action: 'CLOSE',
      coin: coin ?? '',
      signalDate: dayjs().utc().format('YYYY-MM-DD'),
      force: false,
      refPrice: '',
    },
  })
  const action = useWatch({ control: form.control, name: 'action' })

  const submit = form.handleSubmit(async (v) => {
    try {
      const res = await create.mutateAsync({
        action: v.action,
        coin: v.coin,
        side: v.side,
        signalDate: v.action === 'OPEN' ? v.signalDate : undefined,
        refPrice: v.refPrice || undefined,
        rule: v.rule,
        force: v.force,
      })
      toast.success(res.created ? `Đã tạo todo #${res.todo.id}` : `Todo #${res.todo.id} đã có trong hàng đợi`)
      setOpen(false)
      form.reset()
    } catch (err) {
      if (err instanceof ApiError && err.status === 400) form.setError('root', { message: err.message })
      else toast.error(errorMessage(err))
    }
  })

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="outline">
          <PlusIcon />
          Lệnh tay
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={(e) => void submit(e)} className="space-y-4">
          <DialogHeader>
            <DialogTitle>Tạo lệnh tay</DialogTitle>
            <DialogDescription>
              Dùng khi sync không tự sửa được. Áp dụng cùng các kiểm tra như sync; khối lượng lấy từ cấu hình.
            </DialogDescription>
          </DialogHeader>
          <Controller
            control={form.control}
            name="action"
            render={({ field }) => (
              <ToggleGroup
                type="single"
                variant="outline"
                value={field.value}
                onValueChange={(v) => v && field.onChange(v)}
              >
                <ToggleGroupItem value="CLOSE">Đóng lệnh đang giữ</ToggleGroupItem>
                <ToggleGroupItem value="OPEN">Mở lệnh mới</ToggleGroupItem>
              </ToggleGroup>
            )}
          />
          <Field data-invalid={!!form.formState.errors.coin}>
            <FieldLabel>Coin</FieldLabel>
            <Controller
              control={form.control}
              name="coin"
              render={({ field }) => (
                <Select value={field.value} onValueChange={field.onChange}>
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder="Chọn coin" />
                  </SelectTrigger>
                  <SelectContent>
                    {pairs
                      ?.filter((p) => p.status !== 'DISABLED')
                      .map((p) => (
                        <SelectItem key={p.id} value={p.coin}>
                          {p.coin} {p.position ? `· đang ${p.position.side}` : ''}
                        </SelectItem>
                      ))}
                  </SelectContent>
                </Select>
              )}
            />
            <FieldError>{form.formState.errors.coin?.message}</FieldError>
          </Field>
          {action === 'OPEN' && (
            <>
              <Controller
                control={form.control}
                name="side"
                render={({ field }) => (
                  <ToggleGroup
                    type="single"
                    variant="outline"
                    value={field.value}
                    onValueChange={(v) => v && field.onChange(v)}
                  >
                    <ToggleGroupItem value="LONG">LONG</ToggleGroupItem>
                    <ToggleGroupItem value="SHORT">SHORT</ToggleGroupItem>
                  </ToggleGroup>
                )}
              />
              <Field data-invalid={!!form.formState.errors.signalDate}>
                <FieldLabel htmlFor="signal-date">Ngày tín hiệu (nến 1D, UTC)</FieldLabel>
                <Input id="signal-date" type="date" {...form.register('signalDate')} />
                <FieldError>{form.formState.errors.signalDate?.message}</FieldError>
              </Field>
              <div className="flex items-center gap-2 text-sm">
                <Controller
                  control={form.control}
                  name="force"
                  render={({ field }) => (
                    <Checkbox
                      id="force"
                      checked={field.value}
                      onCheckedChange={(v) => field.onChange(v === true)}
                    />
                  )}
                />
                <Label htmlFor="force">Bỏ qua giới hạn tuổi tín hiệu (force)</Label>
              </div>
            </>
          )}
          <Field data-invalid={!!form.formState.errors.refPrice}>
            <FieldLabel htmlFor="ref-price">Giá tham chiếu (tuỳ chọn)</FieldLabel>
            <Input
              id="ref-price"
              inputMode="decimal"
              placeholder="giá open của nến tín hiệu"
              {...form.register('refPrice')}
            />
            <FieldDescription>Dùng để đo trượt giá so với chiến lược.</FieldDescription>
          </Field>
          {form.formState.errors.root && (
            <p className="text-sm text-destructive">{form.formState.errors.root.message}</p>
          )}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setOpen(false)}>
              Huỷ
            </Button>
            <Button type="submit" disabled={form.formState.isSubmitting}>
              {form.formState.isSubmitting && <Loader2Icon className="animate-spin" />}
              Tạo todo
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
