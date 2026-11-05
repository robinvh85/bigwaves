import Decimal from 'decimal.js-light'

/**
 * pct % of total, rounded down to the pair's quantity step (or to total's decimals when the step is
 * unknown), as a plain decimal string.
 */
export function amountForPercent(total: string, pct: number, stepSize?: string): string {
  const t = new Decimal(total)
  const raw = t.times(pct).div(100)
  const step = stepSize ? new Decimal(stepSize) : null
  if (step && step.gt(0)) {
    return raw.div(step).toDecimalPlaces(0, Decimal.ROUND_DOWN).times(step).toFixed(step.decimalPlaces())
  }
  const dp = t.decimalPlaces()
  return raw.toDecimalPlaces(dp, Decimal.ROUND_DOWN).toFixed(dp)
}

/** amount as a share of total, 0–100, or 0 when either is not a positive number. */
export function percentOfTotal(amount: string, total: string): number {
  if (!/^\d+(\.\d+)?$/.test(amount)) return 0
  const t = new Decimal(total)
  if (t.lte(0)) return 0
  const pct = new Decimal(amount).div(t).times(100).toNumber()
  return Math.min(100, Math.max(0, pct))
}
