import { useQuery } from '@tanstack/react-query'
import { Loader2Icon, SearchCheckIcon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/shared/components/confirm-dialog'
import { Pct, Pnl, Price, Side } from '@/shared/components/numbers'
import { formatDate, formatDateTime } from '@/shared/lib/date'
import { Button } from '@/shared/ui/button'
import { Input } from '@/shared/ui/input'
import { Label } from '@/shared/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui/select'
import { Separator } from '@/shared/ui/separator'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/shared/ui/sheet'
import { Switch } from '@/shared/ui/switch'

import { type Ema1dPairRow, type Ema1dPairUpdate, pairsQuery, useCheckPairs, useUpdatePair } from '../api'
import { BasketStatusBadge, WantedLabel } from './badges'

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 py-1 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="text-right font-medium">{children}</span>
    </div>
  )
}

const STATUS_HELP: Record<Ema1dPairRow['status'], string> = {
  ACTIVE: 'Nhận cả tín hiệu vào và thoát.',
  EXIT_ONLY: 'Đã loại khỏi rổ: chỉ đóng lệnh đang giữ, không vào mới.',
  DISABLED: 'Bỏ qua hoàn toàn (kể cả đóng lệnh). Chỉ dùng khi đã xử lý tay.',
}

export function BasketPairSheet({ id, onClose }: { id?: number; onClose: () => void }) {
  const { data: pairs } = useQuery(pairsQuery)
  const pair = pairs?.find((p) => p.id === id)
  return (
    <Sheet open={id !== undefined} onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="w-full overflow-y-auto sm:max-w-lg">
        {pair && <Detail key={pair.id} pair={pair} />}
      </SheetContent>
    </Sheet>
  )
}

function Detail({ pair }: { pair: Ema1dPairRow }) {
  const update = useUpdatePair(pair.id)
  const check = useCheckPairs()
  const [pending, setPending] = useState<{
    title: string
    description: string
    body: Ema1dPairUpdate
  } | null>(null)
  const ask = (title: string, description: string, body: Ema1dPairUpdate) =>
    setPending({ title, description, body })

  return (
    <>
      <SheetHeader>
        <SheetTitle className="flex items-center gap-2">
          <span className="font-mono">{pair.coin}</span> <BasketStatusBadge status={pair.status} />
        </SheetTitle>
        <SheetDescription>
          {pair.pairName} · nhóm {pair.pairGroup === 'CORE' ? 'lõi' : 'nhạy'} · thêm vào rổ{' '}
          {formatDate(pair.addedOn)}
        </SheetDescription>
      </SheetHeader>
      <div className="space-y-4 px-4 pb-6">
        <div className="rounded-lg border p-3">
          <Row label="Giá hiện tại">
            <Price value={pair.currentPrice} />
          </Row>
          <Row label="Đang giữ">
            {pair.position ? (
              <span className="inline-flex items-center gap-2">
                <Side side={pair.position.side} /> từ {formatDate(pair.position.signalDate)}
              </span>
            ) : (
              <span className="text-muted-foreground">không</span>
            )}
          </Row>
          {pair.position && (
            <Row label="Lãi/lỗ chưa chốt">
              <Pnl value={pair.position.unrealizedProfitUsdt} />{' '}
              <Pct value={pair.position.unrealizedPercent} className="text-xs" />
            </Row>
          )}
          <Row label="Sync muốn">
            {pair.wanted ? (
              <span>
                <WantedLabel state={pair.wanted.state} />
                {pair.wanted.entryDate && (
                  <span className="text-muted-foreground"> từ {formatDate(pair.wanted.entryDate)}</span>
                )}
              </span>
            ) : (
              '—'
            )}
          </Row>
          {pair.pendingTodos.length > 0 && <Row label="Đang chờ">{pair.pendingTodos.join(', ')}</Row>}
          {pair.mismatch && (
            <p className="mt-2 text-sm text-warning">
              Vị thế đang lệch với chiến lược và không có todo nào sửa.
            </p>
          )}
        </div>

        <div className="space-y-3">
          <h3 className="text-sm font-medium">Thiết lập</h3>
          <div className="grid gap-2">
            <Label>Trạng thái trong rổ</Label>
            <Select
              value={pair.status}
              onValueChange={(v) =>
                ask(`Đổi ${pair.coin} sang ${v}?`, STATUS_HELP[v as Ema1dPairRow['status']], {
                  status: v as Ema1dPairRow['status'],
                })
              }
            >
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="ACTIVE">Trong rổ</SelectItem>
                <SelectItem value="EXIT_ONLY">Chỉ thoát</SelectItem>
                <SelectItem value="DISABLED">Tắt</SelectItem>
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">{STATUS_HELP[pair.status]}</p>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-2">
              <Label>Nhóm (khối lượng)</Label>
              <Select
                value={pair.pairGroup}
                onValueChange={(v) => update.mutate({ pairGroup: v as 'CORE' | 'SENSITIVE' })}
              >
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="CORE">Lõi</SelectItem>
                  <SelectItem value="SENSITIVE">Nhạy</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>Thoát LONG theo</Label>
              <Select
                value={pair.longExitRule ?? 'DIV4H'}
                onValueChange={(v) => update.mutate({ longExitRule: v })}
              >
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="DIV4H">DIV4H (phân kỳ 4H)</SelectItem>
                  <SelectItem value="XO">XO (cắt ngược)</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="flex items-center justify-between rounded-lg border p-3">
            <Label htmlFor="allow-long">Cho phép LONG</Label>
            <Switch
              id="allow-long"
              checked={pair.allowLong}
              onCheckedChange={(v) => update.mutate({ allowLong: v })}
            />
          </div>
          <div className="flex items-center justify-between rounded-lg border p-3">
            <div>
              <Label htmlFor="allow-short">Cho phép SHORT</Label>
              {pair.allowShort && !pair.shortSupported && (
                <p className="text-xs text-warning">
                  Binance hiện không cho vay {pair.coin}: SHORT sẽ bị bỏ qua.
                </p>
              )}
            </div>
            <Switch
              id="allow-short"
              checked={pair.allowShort}
              onCheckedChange={(v) => update.mutate({ allowShort: v })}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="signal-from">Chỉ theo tín hiệu từ ngày</Label>
            <Input
              id="signal-from"
              type="date"
              defaultValue={pair.signalFrom ?? ''}
              onBlur={(e) => {
                if (e.target.value !== (pair.signalFrom ?? '')) update.mutate({ signalFrom: e.target.value })
              }}
            />
            <p className="text-xs text-muted-foreground">
              Để trống: theo toàn bộ lịch sử. Coin mới thêm: chỉ vào ở tín hiệu mới.
            </p>
          </div>
        </div>

        <Separator />
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-medium">Kiểm tra trên Binance</h3>
            <Button
              size="sm"
              variant="outline"
              disabled={check.isPending}
              onClick={() =>
                check.mutate([pair.id], {
                  onSuccess: (r) =>
                    toast.success(
                      r.items[0]?.shortSupported ? 'SHORT được hỗ trợ' : 'SHORT KHÔNG được hỗ trợ',
                    ),
                })
              }
            >
              {check.isPending ? <Loader2Icon className="animate-spin" /> : <SearchCheckIcon />}
              Kiểm tra
            </Button>
          </div>
          <p className="text-sm text-muted-foreground">{pair.checkNote || 'Chưa kiểm tra.'}</p>
          {pair.checkedAt && (
            <p className="text-xs text-muted-foreground">Lần cuối {formatDateTime(pair.checkedAt)}</p>
          )}
        </div>
      </div>
      {pending && (
        <ConfirmDialog
          open
          onOpenChange={(o) => !o && setPending(null)}
          title={pending.title}
          description={pending.description}
          onConfirm={() => update.mutateAsync(pending.body)}
        />
      )}
    </>
  )
}
