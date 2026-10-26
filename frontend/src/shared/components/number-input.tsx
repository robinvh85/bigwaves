import type { ComponentProps } from 'react'

import { sanitizeDecimal } from '@/shared/lib/decimal-input'
import { Input } from '@/shared/ui/input'

type Props = Omit<ComponentProps<typeof Input>, 'type' | 'inputMode'> & { allowNegative?: boolean }

/**
 * Text input for decimal strings (prices, amounts, percentages). Letters and other characters are
 * dropped as they are typed or pasted, so the form only ever sees digits and one ".". Works with
 * react-hook-form's register(): the value is cleaned before the field's onChange runs.
 */
export function NumberInput({ allowNegative = false, onChange, ...props }: Props) {
  return (
    <Input
      inputMode="decimal"
      autoComplete="off"
      {...props}
      onChange={(e) => {
        const el = e.currentTarget
        const clean = sanitizeDecimal(el.value, allowNegative)
        if (clean !== el.value) {
          const caret = Math.max(0, (el.selectionStart ?? clean.length) - (el.value.length - clean.length))
          el.value = clean
          el.setSelectionRange(caret, caret)
        }
        onChange?.(e)
      }}
    />
  )
}
