/** Margin level color: below 1.3 is danger, below 1.5 a warning (same thresholds as the monitor). */
export function marginLevelClass(level: string | undefined) {
  const v = Number(level)
  if (!level || Number.isNaN(v)) return 'text-muted-foreground'
  if (v < 1.3) return 'text-pnl-down'
  if (v < 1.5) return 'text-warning'
  return 'text-pnl-up'
}
