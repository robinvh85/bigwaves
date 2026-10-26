/** Where a pair's orders go on Binance. Stored as two flags on the pair; edited as one choice. */
export type AccountType = 'CROSS' | 'ISOLATED' | 'SPOT'

export const ACCOUNT_OPTIONS: { value: AccountType; label: string }[] = [
  { value: 'CROSS', label: 'Cross margin' },
  { value: 'ISOLATED', label: 'Isolated' },
  { value: 'SPOT', label: 'Spot' },
]

export const ACCOUNT_HINT = 'EMA1D needs cross margin. DCA also works on spot.'

export function accountOf(p: { isSpot: boolean; isIsolated: boolean }): AccountType {
  return p.isSpot ? 'SPOT' : p.isIsolated ? 'ISOLATED' : 'CROSS'
}

export function accountFlags(a: AccountType): { isSpot: boolean; isIsolated: boolean } {
  return { isSpot: a === 'SPOT', isIsolated: a === 'ISOLATED' }
}

export function accountLabel(a: AccountType): string {
  return ACCOUNT_OPTIONS.find((o) => o.value === a)?.label ?? a
}
