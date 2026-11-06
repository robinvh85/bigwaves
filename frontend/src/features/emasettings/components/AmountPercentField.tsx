import { NumberInput } from '@/shared/components/number-input'
import { Slider } from '@/shared/ui/slider'

import { amountForPercent, percentOfTotal } from '../amount-percent'

const MARKS = [25, 50, 75]

type Props = {
  id: string
  /** The trade's whole amount: 100%. */
  total: string
  stepSize?: string
  value: string
  onChange: (amount: string) => void
}

/** Amount input with a slider and 25/50/75% shortcuts, all relative to total. */
export function AmountPercentField({ id, total, stepSize, value, onChange }: Props) {
  const pct = percentOfTotal(value, total)
  const set = (p: number) => onChange(amountForPercent(total, p, stepSize))

  return (
    <div className="grid gap-3">
      <div className="flex items-center gap-2">
        <NumberInput id={id} value={value} onChange={(e) => onChange(e.target.value)} />
        <span className="num w-14 shrink-0 text-right text-sm text-muted-foreground">{Math.round(pct)}%</span>
      </div>
      <div className="grid gap-1.5 px-1.5">
        <Slider
          aria-label="Share of the trade amount"
          min={0}
          max={100}
          step={1}
          value={[Math.round(pct)]}
          onValueChange={([p]) => p !== undefined && set(p)}
        />
        <div className="relative h-5">
          {MARKS.map((m) => (
            <button
              key={m}
              type="button"
              onClick={() => set(m)}
              style={{ left: `${m}%` }}
              className="absolute -translate-x-1/2 rounded px-1 text-xs text-muted-foreground hover:bg-muted hover:text-foreground"
            >
              {m}%
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}
