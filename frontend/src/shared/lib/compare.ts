/** Plain numbers and decimal strings ("-1.5", "0.0012", "12%"); anything else is not a number. */
function asNumber(v: unknown): number | null {
  if (typeof v === 'number') return Number.isNaN(v) ? null : v
  if (typeof v !== 'string') return null
  const t = v.trim().replace(/%$/, '')
  return /^[+-]?(\d+\.?\d*|\.\d+)(e[+-]?\d+)?$/i.test(t) ? Number(t) : null
}

const asText = (v: unknown) =>
  typeof v === 'string' ? v : typeof v === 'number' || typeof v === 'boolean' ? String(v) : ''

const isEmpty = (v: unknown) => v === null || v === undefined || v === ''

/**
 * Sort order for table cells. Money and prices come from the API as decimal strings, so numbers
 * in strings compare by value ("-5" < "3", "0.0012" < "0.012"); other text compares naturally.
 * Ordering only: never use this to do arithmetic on money.
 */
export function compareCellValues(a: unknown, b: unknown): number {
  if (isEmpty(a) || isEmpty(b)) return isEmpty(a) ? (isEmpty(b) ? 0 : -1) : 1
  const na = asNumber(a)
  const nb = asNumber(b)
  if (na !== null && nb !== null) return na < nb ? -1 : na > nb ? 1 : 0
  return asText(a).localeCompare(asText(b), undefined, { numeric: true, sensitivity: 'base' })
}
