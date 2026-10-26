import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'

import { balancesQuery } from '@/features/account/api'
import { marginLevelClass } from '@/features/account/margin'
import { jobsQuery } from '@/features/jobs/api'
import { pairsQuery } from '@/features/pairs/api'
import { Money } from '@/shared/components/numbers'
import { PageHeader } from '@/shared/components/states'
import { StatCard } from '@/shared/components/stat-card'
import { errorMessage } from '@/shared/api/errors'
import { fromNow } from '@/shared/lib/date'
import { formatMoney } from '@/shared/lib/format'
import { cn } from '@/shared/lib/utils'

export const Route = createFileRoute('/_app/')({
  staticData: { title: 'Tổng quan' },
  component: Overview,
})

function Overview() {
  const balances = useQuery(balancesQuery)
  const jobs = useQuery(jobsQuery)
  const pairs = useQuery(pairsQuery({ status: 'ACTIVE' }))
  const failedJobs = jobs.data?.filter((j) => j.lastRun?.status === 'FAILED') ?? []

  return (
    <>
      <PageHeader title="Tổng quan" />
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
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
          label="Job lỗi"
          loading={jobs.isLoading}
          value={<span className={cn(failedJobs.length > 0 && 'text-destructive')}>{failedJobs.length}</span>}
          hint={
            <Link to="/admin/jobs" className="underline-offset-2 hover:underline">
              {failedJobs.length > 0 ? failedJobs.map((j) => j.name).join(', ') : 'tất cả đang ổn'}
            </Link>
          }
        />
        <StatCard
          label="Pair đang hoạt động"
          loading={pairs.isLoading}
          value={pairs.data?.length ?? '—'}
          hint={
            <Link to="/admin/pairs" className="underline-offset-2 hover:underline">
              quản lý pairs
            </Link>
          }
        />
      </div>
    </>
  )
}
