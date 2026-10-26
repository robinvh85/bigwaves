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
  staticData: { title: 'Overview' },
  component: Overview,
})

function Section({ title, to, children }: { title: string; to?: string; children: ReactNode }) {
  return (
    <section className="mb-6">
      <div className="mb-2 flex items-center justify-between">
        <h2 className="font-medium">{title}</h2>
        {to && (
          <Link to={to} className="text-sm text-muted-foreground underline-offset-2 hover:underline">
            Details →
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
      <PageHeader title="Overview" />
      <Section title="Binance account">
        <StatCard
          label="Net assets"
          loading={balances.isLoading}
          value={balances.data ? <Money value={balances.data.netTotalUsdt} /> : '—'}
          hint={
            balances.error
              ? errorMessage(balances.error)
              : balances.data && `updated ${fromNow(balances.data.fetchedAt)}`
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
          hint="warning below 1.5, alert below 1.3"
        />
        <StatCard
          label="Cross USDT available"
          loading={balances.isLoading}
          value={<Money value={balances.data?.crossUsdtFree} />}
        />
        <StatCard
          label="Failed jobs"
          loading={jobs.isLoading}
          value={<span className={cn(failedJobs.length > 0 && 'text-destructive')}>{failedJobs.length}</span>}
          hint={
            <Link to="/admin/jobs" className="underline-offset-2 hover:underline">
              {failedJobs.length > 0 ? failedJobs.map((j) => j.name).join(', ') : 'all good'}
            </Link>
          }
        />
      </Section>
      <Section title="EMA1D" to="/ema1d">
        <StatCard
          label="Strategy capital"
          loading={ema1d.isLoading}
          value={<Money value={ema1d.data?.capital.equityUsdt} />}
        />
        <StatCard
          label="Unrealized PnL"
          loading={ema1d.isLoading}
          value={<Pnl value={ema1d.data?.capital.unrealizedProfitUsdt} />}
          hint={
            ema1d.data && `${ema1d.data.exposure.longCount} LONG · ${ema1d.data.exposure.shortCount} SHORT`
          }
        />
        <StatCard
          label="Failed orders"
          loading={ema1d.isLoading}
          value={
            <span className={cn(failedTodos.length > 0 && 'text-destructive')}>{failedTodos.length}</span>
          }
          hint={
            <Link to="/ema1d/todos" className="underline-offset-2 hover:underline">
              queue
            </Link>
          }
        />
        <StatCard
          label="Status"
          loading={ema1d.isLoading}
          value={
            <span className="text-base">
              {ema1d.data?.enabled ? (ema1d.data.pauseOpen ? 'Closing only' : 'Running') : 'Execution off'}
            </span>
          }
          hint={ema1d.data?.lastSync ? `sync ${ema1d.data.lastSync.date}` : 'no sync yet'}
        />
      </Section>
      <Section title="DCA" to="/dca">
        <StatCard label="Active ladders" loading={dca.isLoading} value={activeDca.length} />
        <StatCard
          label="Capital held"
          loading={dca.isLoading}
          value={<Money value={sum(dca.data, 'capitalUsdt')} />}
        />
        <StatCard
          label="Realized"
          loading={dca.isLoading}
          value={<Pnl value={sum(dca.data, 'realizedProfit')} />}
        />
        <StatCard
          label="Unrealized"
          loading={dca.isLoading}
          value={<Pnl value={sum(dca.data, 'unrealizedProfit')} />}
        />
      </Section>
    </>
  )
}
