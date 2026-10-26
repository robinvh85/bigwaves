import Decimal from 'decimal.js-light'

/** Money and quantities come from the API as decimal strings; never parse them with Number for math. */
export type DecimalString = string

const LOCALE = 'en-US'

function toDecimal(v: DecimalString | number | null | undefined): Decimal | null {
  if (v === null || v === undefined || v === '') return null
  try {
    return new Decimal(v)
  } catch {
    return null
  }
}

function group(d: Decimal, dp: number): string {
  const [int, frac] = d.toFixed(dp).split('.')
  const sign = int!.startsWith('-') ? '-' : ''
  const digits = sign ? int!.slice(1) : int!
  const grouped = new Intl.NumberFormat(LOCALE).format(BigInt(digits))
  return frac ? `${sign}${grouped}.${frac}` : `${sign}${grouped}`
}

/** 1234.5 → "1,234.50". Default 2 decimals, for USDT amounts. */
export function formatMoney(v: DecimalString | number | null | undefined, dp = 2): string {
  const d = toDecimal(v)
  return d ? group(d, dp) : '—'
}

/**
 * Prices span many magnitudes (BTC 60000, PEPE 0.00001), so the number of decimals follows the
 * size of the value unless `dp` is given.
 */
export function formatPrice(v: DecimalString | number | null | undefined, dp?: number): string {
  const d = toDecimal(v)
  if (!d) return '—'
  if (dp !== undefined) return group(d, dp)
  const abs = d.abs()
  if (abs.isZero()) return '0'
  if (abs.gte(1000)) return group(d, 2)
  if (abs.gte(1)) return group(d, 4)
  // Keep 4 significant digits for small prices.
  const lead = Math.ceil(-Math.log10(abs.toNumber()))
  return group(d, Math.min(lead + 3, 10))
}

/** Quantity rounded down to a step size string such as "0.001" or "1". */
export function formatQty(v: DecimalString | number | null | undefined, stepSize?: DecimalString): string {
  const d = toDecimal(v)
  if (!d) return '—'
  if (!stepSize) return group(d, Math.min(d.decimalPlaces(), 8))
  const step = new Decimal(stepSize)
  const dp = step.decimalPlaces()
  return group(d.toDecimalPlaces(dp, Decimal.ROUND_DOWN), dp)
}

/** 0.0512 → "+5.12%" when `ratio`, or 5.12 → "+5.12%" when the value is already a percent. */
export function formatPct(
  v: DecimalString | number | null | undefined,
  { ratio = false, dp = 2, sign = true }: { ratio?: boolean; dp?: number; sign?: boolean } = {},
): string {
  let d = toDecimal(v)
  if (!d) return '—'
  if (ratio) d = d.times(100)
  const s = group(d, dp)
  return sign && d.gt(0) ? `+${s}%` : `${s}%`
}

/** -1, 0 or 1, for picking profit/loss colors. */
export function signOf(v: DecimalString | number | null | undefined): -1 | 0 | 1 {
  const d = toDecimal(v)
  if (!d || d.isZero()) return 0
  return d.isNegative() ? -1 : 1
}
