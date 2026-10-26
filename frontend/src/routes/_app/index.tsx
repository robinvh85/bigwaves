import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import type { ReactNode } from 'react'

import { balancesQuery } from '@/features/account/api'
import { marginLevelClass } from '@/features/account/margin'
import { dcaListQuery } from '@/features/dca/api'
import { statusQuery } from '@/features/ema1d/api'
import { jobsQuery } from '@/features/jobs/api'
import { errorMessage } from '@/shared/api/errors'
import { Money, Pnl } from '@/shared/components/numbers'
import { PageHeader } from '@/shared/components/states'
import { StatCard } from '@/shared/components/stat-card'
import { fromNow } from '@/shared/lib/date'
import { formatMoney } from '@/shared/lib/format'
import { cn } from '@/shared/lib/utils'

export const Route = createFileRoute('/_app/')({
  staticData: { title: 'Tổng quan' },
  component: Overview,
})

function Section({ title, to, children }: { title: string; to?: string; children: ReactNode }) {
  return (
    <section className="mb-6">
      <div className="mb-2 flex items-center justify-between">
        <h2 className="font-medium">{title}</h2>
        {to && (
          <Link to={to} className="text-sm text-muted-foreground underline-offset-2 hover:underline">
            Chi tiết →
          </Link>
        )}
      </div>
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">{children}</div>
    </section>
  )
}

function sum(rows: { [k: string]: unknown }[] | undefined, key: string) {
  return rows ? String(rows.reduce((a, r) => a + Number(r[key] ?? 0), 0)) : undefined
}

function Overview() {
  const balances = useQuery(balancesQuery)
  const ema1d = useQuery(statusQuery)
  const dca = useQuery(dcaListQuery)
  const jobs = useQuery(jobsQuery)
  const failedJobs = jobs.data?.filter((j) => j.lastRun?.status === 'FAILED') ?? []
  const failedTodos = ema1d.data?.todos.filter((t) => t.status === 'FAILED') ?? []
  const activeDca = dca.data?.filter((s) => s.status === 'ACTIVE') ?? []

  return (
    <>
      <PageHeader title="Tổng quan" />
      <Section title="Tài khoản Binance">
        <StatCard
          label="Tài sản ròng"
          loading={balances.isLoading}
          value={balances.data ? <Money value={balances.data.netTotalUsdt} /> : '—'}
          hint={
            balances.error
              ? errorMessage(balances.error)
              : balances.data && `cập nhật ${fromNow(balances.data.fetchedAt)}`
          }
        />
        <StatCard
          label="Margin level"
          loading={balances.isLoading}
          value={
            <span className={cn('num', marginLevelClass(balances.data?.marginLevel))}>
              {formatMoney(balances.data?.marginLevel)}
            </span>
          }
          hint="cảnh báo dưới 1.5, báo động dưới 1.3"
        />
        <StatCard
          label="USDT cross khả dụng"
          loading={balances.isLoading}
          value={<Money value={balances.data?.crossUsdtFree} />}
        />
        <StatCard
          label="Job lỗi"
          loading={jobs.isLoading}
          value={<span className={cn(failedJobs.length > 0 && 'text-destructive')}>{failedJobs.length}</span>}
          hint={
            <Link to="/admin/jobs" className="underline-offset-2 hover:underline">
              {failedJobs.length > 0 ? failedJobs.map((j) => j.name).join(', ') : 'tất cả đang ổn'}
            </Link>
          }
        />
      </Section>
      <Section title="EMA1D" to="/ema1d">
        <StatCard
          label="Vốn chiến lược"
          loading={ema1d.isLoading}
          value={<Money value={ema1d.data?.capital.equityUsdt} />}
        />
        <StatCard
          label="Lãi/lỗ chưa chốt"
          loading={ema1d.isLoading}
          value={<Pnl value={ema1d.data?.capital.unrealizedProfitUsdt} />}
          hint={
            ema1d.data && `${ema1d.data.exposure.longCount} LONG · ${ema1d.data.exposure.shortCount} SHORT`
          }
        />
        <StatCard
          label="Lệnh lỗi"
          loading={ema1d.isLoading}
          value={
            <span className={cn(failedTodos.length > 0 && 'text-destructive')}>{failedTodos.length}</span>
          }
          hint={
            <Link to="/ema1d/todos" className="underline-offset-2 hover:underline">
              hàng đợi
            </Link>
          }
        />
        <StatCard
          label="Trạng thái"
          loading={ema1d.isLoading}
          value={
            <span className="text-base">
              {ema1d.data?.enabled ? (ema1d.data.pauseOpen ? 'Chỉ đóng lệnh' : 'Đang chạy') : 'Tắt thực thi'}
            </span>
          }
          hint={ema1d.data?.lastSync ? `sync ${ema1d.data.lastSync.date}` : 'chưa có sync'}
        />
      </Section>
      <Section title="DCA" to="/dca">
        <StatCard label="Ladder đang chạy" loading={dca.isLoading} value={activeDca.length} />
        <StatCard
          label="Vốn đang giữ"
          loading={dca.isLoading}
          value={<Money value={sum(dca.data, 'capitalUsdt')} />}
        />
        <StatCard
          label="Đã chốt"
          loading={dca.isLoading}
          value={<Pnl value={sum(dca.data, 'realizedProfit')} />}
        />
        <StatCard
          label="Chưa chốt"
          loading={dca.isLoading}
          value={<Pnl value={sum(dca.data, 'unrealizedProfit')} />}
        />
      </Section>
    </>
  )
}
