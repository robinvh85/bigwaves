import { useQuery } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { SearchIcon } from 'lucide-react'
import { z } from 'zod'

import { pairsQuery } from '@/features/pairs/api'
import { pairColumns } from '@/features/pairs/columns'
import { CreatePairDialog } from '@/features/pairs/CreatePairDialog'
import { PairSheet } from '@/features/pairs/PairSheet'
import { DataTable } from '@/shared/components/data-table'
import { sortFromParam, sortToParam } from '@/shared/lib/sort-param'
import { EmptyState, PageHeader } from '@/shared/components/states'
import { Input } from '@/shared/ui/input'
import { ToggleGroup, ToggleGroupItem } from '@/shared/ui/toggle-group'

const search = z.object({
  q: z.string().optional(),
  status: z.enum(['ACTIVE', 'DISABLED']).optional(),
  sort: z.string().optional(),
  pair: z.number().int().optional(),
})

export const Route = createFileRoute('/_app/admin/pairs')({
  staticData: { title: 'Pairs' },
  validateSearch: search,
  component: PairsPage,
})

function PairsPage() {
  const s = Route.useSearch()
  const navigate = useNavigate({ from: Route.fullPath })
  const set = (patch: Partial<z.infer<typeof search>>) =>
    void navigate({ search: (prev) => ({ ...prev, ...patch }), replace: true })
  const { data, isLoading, error, refetch } = useQuery(pairsQuery({ status: s.status }))

  return (
    <>
      <PageHeader
        title="Pairs"
        description="Các cặp giao dịch trên Binance. Bấm vào một dòng để xem chi tiết và ghi chú coin."
        actions={<CreatePairDialog onCreated={(p) => set({ pair: p.id })} />}
      />
      <div className="mb-3 flex flex-col gap-2 sm:flex-row sm:items-center">
        <div className="relative sm:w-72">
          <SearchIcon className="absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Tìm pair…"
            className="pl-8"
            value={s.q ?? ''}
            onChange={(e) => set({ q: e.target.value || undefined })}
            aria-label="Tìm pair"
          />
        </div>
        <ToggleGroup
          type="single"
          variant="outline"
          size="sm"
          value={s.status ?? 'ALL'}
          onValueChange={(v) => {
            if (v) set({ status: v === 'ALL' ? undefined : (v as 'ACTIVE' | 'DISABLED') })
          }}
        >
          <ToggleGroupItem value="ALL">Tất cả</ToggleGroupItem>
          <ToggleGroupItem value="ACTIVE">Active</ToggleGroupItem>
          <ToggleGroupItem value="DISABLED">Disabled</ToggleGroupItem>
        </ToggleGroup>
      </div>
      <DataTable
        columns={pairColumns}
        data={data}
        isLoading={isLoading}
        error={error}
        onRetry={() => void refetch()}
        getRowId={(p) => String(p.id)}
        globalFilter={s.q}
        sorting={sortFromParam(s.sort ?? 'name')}
        onSortingChange={(st) => set({ sort: sortToParam(st) })}
        onRowClick={(p) => set({ pair: p.id })}
        empty={
          s.q || s.status ? undefined : (
            <EmptyState
              title="Chưa có pair nào"
              description="Thêm pair đầu tiên bằng symbol Binance, vd SUNUSDT."
            />
          )
        }
      />
      <PairSheet pairId={s.pair} onClose={() => set({ pair: undefined })} />
    </>
  )
}
