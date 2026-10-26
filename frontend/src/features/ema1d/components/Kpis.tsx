import { marginLevelClass } from '@/features/account/margin'
import { Money, Pnl } from '@/shared/components/numbers'
import { StatCard } from '@/shared/components/stat-card'
import { fromNow } from '@/shared/lib/date'
import { formatMoney, formatPct } from '@/shared/lib/format'
import { cn } from '@/shared/lib/utils'

import type { Ema1dStatus } from '../api'

export function Ema1dKpis({ status }: { status: Ema1dStatus }) {
  const { capital, exposure, marginLevel } = status
  return (
    <div className="mb-6 grid grid-cols-2 gap-3 lg:grid-cols-5">
      <StatCard
        label="Strategy capital"
        value={<Money value={capital.equityUsdt} />}
        hint={
          <>
            budget {formatMoney(capital.budgetUsdt)} + realized{' '}
            <Pnl value={capital.realizedProfitUsdt} unit={null} />
          </>
        }
      />
      <StatCard
        label="Unrealized PnL"
        value={<Pnl value={capital.unrealizedProfitUsdt} />}
        hint={`${status.openPositions.length} open positions`}
      />
      <StatCard
        label="Margin level"
        value={
          <span className={cn('num', marginLevelClass(marginLevel.value))}>
            {formatMoney(marginLevel.value)}
          </span>
        }
        hint={marginLevel.at ? `measured ${fromNow(marginLevel.at)}` : 'not measured yet'}
      />
      <StatCard
        label="LONG"
        value={<Money value={exposure.longUsdt} />}
        hint={`${exposure.longCount} positions · ${formatPct(exposure.longPercent, { sign: false })} of capital`}
      />
      <StatCard
        label="SHORT"
        value={<Money value={exposure.shortUsdt} />}
        hint={`${exposure.shortCount} positions · ${formatPct(exposure.shortPercent, { sign: false })} of capital`}
      />
    </div>
  )
}
