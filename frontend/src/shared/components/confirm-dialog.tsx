import { Loader2Icon } from 'lucide-react'
import { type ReactNode, useState } from 'react'

import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/shared/ui/alert-dialog'
import { Button } from '@/shared/ui/button'
import { Input } from '@/shared/ui/input'
import { Label } from '@/shared/ui/label'

/**
 * Confirmation for actions with real consequences (orders, borrow/repay, deletes).
 * With `typeToConfirm`, the user has to type that text (e.g. the pair name) before confirming.
 * The dialog stays open with a spinner until `onConfirm` resolves, and closes only on success.
 */
export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  children,
  confirmLabel = 'Xác nhận',
  destructive,
  typeToConfirm,
  confirmDisabled,
  onConfirm,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description?: ReactNode
  children?: ReactNode
  confirmLabel?: string
  destructive?: boolean
  typeToConfirm?: string
  /** Keeps the confirm button disabled, e.g. while a form inside the dialog is invalid. */
  confirmDisabled?: boolean
  onConfirm: () => unknown
}) {
  const [typed, setTyped] = useState('')
  const [pending, setPending] = useState(false)
  const blocked = Boolean(confirmDisabled) || (typeToConfirm !== undefined && typed.trim() !== typeToConfirm)

  const close = (next: boolean) => {
    if (pending) return
    if (!next) setTyped('')
    onOpenChange(next)
  }

  const confirm = async () => {
    setPending(true)
    try {
      await onConfirm()
      setTyped('')
      onOpenChange(false)
    } catch {
      // The caller shows the error (toast); keep the dialog open so the user can retry or cancel.
    } finally {
      setPending(false)
    }
  }

  return (
    <AlertDialog open={open} onOpenChange={close}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          {description && <AlertDialogDescription>{description}</AlertDialogDescription>}
        </AlertDialogHeader>
        {children}
        {typeToConfirm !== undefined && (
          <div className="space-y-2">
            <Label htmlFor="confirm-text">
              Gõ <span className="font-mono font-semibold">{typeToConfirm}</span> để xác nhận
            </Label>
            <Input
              id="confirm-text"
              value={typed}
              autoComplete="off"
              onChange={(e) => setTyped(e.target.value)}
              disabled={pending}
            />
          </div>
        )}
        <AlertDialogFooter>
          <AlertDialogCancel disabled={pending}>Huỷ</AlertDialogCancel>
          <Button
            variant={destructive ? 'destructive' : 'default'}
            disabled={blocked || pending}
            onClick={() => void confirm()}
          >
            {pending && <Loader2Icon className="animate-spin" />}
            {confirmLabel}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
