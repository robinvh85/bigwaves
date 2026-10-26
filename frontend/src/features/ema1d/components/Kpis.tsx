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
        label="Vốn chiến lược"
        value={<Money value={capital.equityUsdt} />}
        hint={
          <>
            ngân sách {formatMoney(capital.budgetUsdt)} + đã chốt{' '}
            <Pnl value={capital.realizedProfitUsdt} unit={null} />
          </>
        }
      />
      <StatCard
        label="Lãi/lỗ chưa chốt"
        value={<Pnl value={capital.unrealizedProfitUsdt} />}
        hint={`${status.openPositions.length} lệnh đang mở`}
      />
      <StatCard
        label="Margin level"
        value={
          <span className={cn('num', marginLevelClass(marginLevel.value))}>
            {formatMoney(marginLevel.value)}
          </span>
        }
        hint={marginLevel.at ? `đo ${fromNow(marginLevel.at)}` : 'chưa có số đo'}
      />
      <StatCard
        label="LONG"
        value={<Money value={exposure.longUsdt} />}
        hint={`${exposure.longCount} lệnh · ${formatPct(exposure.longPercent, { sign: false })} vốn`}
      />
      <StatCard
        label="SHORT"
        value={<Money value={exposure.shortUsdt} />}
        hint={`${exposure.shortCount} lệnh · ${formatPct(exposure.shortPercent, { sign: false })} vốn`}
      />
    </div>
  )
}
