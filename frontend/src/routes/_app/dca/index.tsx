import { useQuery } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { createColumnHelper } from '@tanstack/react-table'

import { type DcaSetting, dcaListQuery } from '@/features/dca/api'
import { SettingStatusBadge } from '@/features/dca/components/badges'
import { CreateLadderDialog } from '@/features/dca/components/CreateLadderDialog'
import { DataTable } from '@/shared/components/data-table'
import { Money, Pnl, Price } from '@/shared/components/numbers'
import { EmptyState, PageHeader } from '@/shared/components/states'
import { StatCard } from '@/shared/components/stat-card'

export const Route = createFileRoute('/_app/dca/')({
  staticData: { title: 'Danh sách' },
  component: DcaList,
})

const col = createColumnHelper<DcaSetting>()
const num = (v?: string) => (v ? Number(v) : -Infinity)
const columns = [
  col.accessor('pairName', {
    header: 'Pair',
    cell: (c) => <span className="font-mono font-medium">{c.getValue()}</span>,
    meta: { mobile: 'title' },
  }),
  col.accessor('status', {
    header: 'Trạng thái',
    cell: (c) => <SettingStatusBadge status={c.getValue()} />,
    meta: { mobile: 'badge' },
  }),
  col.accessor('account', {
    header: 'Tài khoản',
    cell: (c) => <span className="text-muted-foreground">{c.getValue()}</span>,
    meta: { mobile: 'hidden' },
  }),
  col.accessor((s) => num(s.currentPrice), {
    id: 'price',
    header: 'Giá',
    cell: (c) => <Price value={c.row.original.currentPrice} />,
    meta: { align: 'right' },
  }),
  col.accessor((s) => num(s.nextBuyPrice), {
    id: 'nextBuy',
    header: 'Mua kế tiếp',
    cell: (c) => <Price value={c.row.original.nextBuyPrice} className="text-primary" />,
    meta: { align: 'right' },
  }),
  col.accessor((s) => num(s.nextSellPrice), {
    id: 'nextSell',
    header: 'Bán kế tiếp',
    cell: (c) => <Price value={c.row.original.nextSellPrice} className="text-long" />,
    meta: { align: 'right' },
  }),
  col.display({
    id: 'counts',
    header: 'Nấc (chờ / giữ)',
    cell: (c) => (
      <span className="num">
        {c.row.original.waitingCount} / {c.row.original.holdingCount}
      </span>
    ),
  }),
  col.accessor((s) => num(s.capitalUsdt), {
    id: 'capital',
    header: 'Vốn đang giữ',
    cell: (c) => <Money value={c.row.original.capitalUsdt} />,
    meta: { align: 'right' },
  }),
  col.accessor((s) => num(s.unrealizedProfit) + num(s.realizedProfit), {
    id: 'pnl',
    header: 'Lãi/lỗ (chốt + chưa chốt)',
    cell: (c) => (
      <div className="flex flex-col items-end">
        <Pnl value={c.row.original.realizedProfit} />
        <Pnl value={c.row.original.unrealizedProfit} className="text-xs" />
      </div>
    ),
    meta: { align: 'right', mobile: 'primary' },
  }),
]

function sum(rows: DcaSetting[] | undefined, key: 'capitalUsdt' | 'realizedProfit' | 'unrealizedProfit') {
  return rows ? String(rows.reduce((a, r) => a + Number(r[key]), 0)) : undefined
}

function DcaList() {
  const navigate = useNavigate()
  const { data, isLoading, error, refetch } = useQuery(dcaListQuery)
  return (
    <>
      <PageHeader
        title="DCA"
        description="Thang lệnh mua do anh đặt: hệ thống chỉ canh giá (5 phút/lần) và khớp lệnh market."
        actions={<CreateLadderDialog />}
      />
      <div className="mb-4 grid grid-cols-2 gap-3 lg:grid-cols-3">
        <StatCard
          label="Vốn đang giữ"
          loading={isLoading}
          value={<Money value={sum(data, 'capitalUsdt')} />}
        />
        <StatCard label="Đã chốt" loading={isLoading} value={<Pnl value={sum(data, 'realizedProfit')} />} />
        <StatCard
          label="Chưa chốt"
          loading={isLoading}
          value={<Pnl value={sum(data, 'unrealizedProfit')} />}
        />
      </div>
      <DataTable
        columns={columns}
        data={data}
        isLoading={isLoading}
        error={error}
        onRetry={() => void refetch()}
        getRowId={(s) => String(s.id)}
        onRowClick={(s) => void navigate({ to: '/dca/$id', params: { id: String(s.id) } })}
        empty={
          <EmptyState
            title="Chưa có ladder nào"
            description="Tạo ladder đầu tiên, rồi thêm các nấc mua/bán."
          />
        }
      />
    </>
  )
}
