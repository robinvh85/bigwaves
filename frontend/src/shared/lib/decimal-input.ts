/**
 * Keeps only what a decimal number can contain: digits, one "." (a "," is read as "."), and a
 * leading "-" when negatives are allowed. Used on every keystroke and paste of a numeric input.
 */
export function sanitizeDecimal(raw: string, allowNegative = false): string {
  let out = ''
  let dot = false
  for (const ch of raw.replace(/,/g, '.')) {
    if (ch >= '0' && ch <= '9') out += ch
    else if (ch === '.' && !dot) {
      dot = true
      out += ch
    } else if (ch === '-' && allowNegative && out === '') out += ch
  }
  return out
}
