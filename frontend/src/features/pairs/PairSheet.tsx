import { useQuery } from '@tanstack/react-query'
import { Trash2Icon } from 'lucide-react'
import { useState } from 'react'

import { ConfirmDialog } from '@/shared/components/confirm-dialog'
import { Price, Qty } from '@/shared/components/numbers'
import { ErrorState, SkeletonRows } from '@/shared/components/states'
import { formatDateTime, fromNow } from '@/shared/lib/date'
import { formatPct } from '@/shared/lib/format'
import { Button } from '@/shared/ui/button'
import { Separator } from '@/shared/ui/separator'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/shared/ui/sheet'
import { Switch } from '@/shared/ui/switch'
import { Label } from '@/shared/ui/label'

import { pairQuery, useDeletePair, useUpdatePair } from './api'
import { CoinDescription } from './CoinDescription'
import { AccountTypeLabel, PairStatusBadge } from './PairStatusBadge'

function Info({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 py-1 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="text-right font-medium">{children}</span>
    </div>
  )
}

export function PairSheet({ pairId, onClose }: { pairId: number | undefined; onClose: () => void }) {
  return (
    <Sheet
      open={pairId !== undefined}
      onOpenChange={(o) => {
        if (!o) onClose()
      }}
    >
      <SheetContent className="w-full overflow-y-auto sm:max-w-lg">
        {pairId !== undefined && <PairDetail id={pairId} onDeleted={onClose} />}
      </SheetContent>
    </Sheet>
  )
}

function PairDetail({ id, onDeleted }: { id: number; onDeleted: () => void }) {
  const { data: pair, error, refetch } = useQuery(pairQuery(id))
  const update = useUpdatePair(id)
  const del = useDeletePair()
  const [confirmDelete, setConfirmDelete] = useState(false)

  if (error) {
    return (
      <div className="p-4">
        <ErrorState error={error} onRetry={() => void refetch()} />
      </div>
    )
  }
  if (!pair) {
    return (
      <div className="p-4">
        <SkeletonRows rows={8} />
      </div>
    )
  }

  return (
    <>
      <SheetHeader>
        <SheetTitle className="flex items-center gap-2 font-mono">
          {pair.name} <PairStatusBadge status={pair.status} />
        </SheetTitle>
        <SheetDescription>
          {pair.coin.name} / {pair.quoteAsset} ·{' '}
          <AccountTypeLabel isSpot={pair.isSpot} isIsolated={pair.isIsolated} />
        </SheetDescription>
      </SheetHeader>

      <div className="space-y-4 px-4 pb-6">
        <div className="rounded-lg border p-3">
          <Info label="Giá hiện tại">
            <Price value={pair.currentPrice} />
          </Info>
          <Info label="Cập nhật giá">
            <span title={formatDateTime(pair.priceUpdatedAt)}>{fromNow(pair.priceUpdatedAt)}</span>
          </Info>
          <Info label="Lãi vay / năm">
            {pair.interestRate ? formatPct(pair.interestRate, { sign: false }) : '—'}
          </Info>
          <Info label="Step size">
            <Qty value={pair.stepSize} />
          </Info>
          <Info label="Min notional">{pair.minNotional ? `${pair.minNotional} USDT` : '—'}</Info>
        </div>

        <div className="flex items-center justify-between rounded-lg border p-3">
          <div>
            <Label htmlFor="pair-active">Đang hoạt động</Label>
            <p className="text-xs text-muted-foreground">
              Pair tắt vẫn được cập nhật giá, nhưng chiến lược không mở lệnh mới.
            </p>
          </div>
          <Switch
            id="pair-active"
            checked={pair.status === 'ACTIVE'}
            disabled={update.isPending}
            onCheckedChange={(on) => update.mutate({ status: on ? 'ACTIVE' : 'DISABLED' })}
          />
        </div>

        <Separator />
        <CoinDescription key={pair.coin.id} pair={pair} />
        <Separator />

        <Button variant="outline" className="w-full text-destructive" onClick={() => setConfirmDelete(true)}>
          <Trash2Icon />
          Xoá pair
        </Button>
      </div>

      <ConfirmDialog
        open={confirmDelete}
        onOpenChange={setConfirmDelete}
        destructive
        title={`Xoá ${pair.name}?`}
        description="Pair bị ẩn khỏi danh sách và không nhận lệnh mới. Lịch sử giao dịch vẫn được giữ. Thêm lại cùng symbol sẽ khôi phục nó."
        typeToConfirm={pair.name}
        confirmLabel="Xoá"
        onConfirm={async () => {
          await del.mutateAsync(pair.id)
          onDeleted()
        }}
      />
    </>
  )
}
