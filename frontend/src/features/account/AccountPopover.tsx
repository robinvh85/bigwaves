import { useQuery, useQueryClient } from '@tanstack/react-query'
import { RefreshCwIcon, WalletIcon } from 'lucide-react'
import { useState } from 'react'

import { api, unwrap } from '@/shared/api/client'
import { ConfirmDialog } from '@/shared/components/confirm-dialog'
import { Money } from '@/shared/components/numbers'
import { ErrorState } from '@/shared/components/states'
import { fromNow } from '@/shared/lib/date'
import { formatMoney } from '@/shared/lib/format'
import { cn } from '@/shared/lib/utils'
import { Button } from '@/shared/ui/button'
import { Input } from '@/shared/ui/input'
import { Label } from '@/shared/ui/label'
import { Popover, PopoverContent, PopoverTrigger } from '@/shared/ui/popover'
import { Separator } from '@/shared/ui/separator'
import { Skeleton } from '@/shared/ui/skeleton'

import { type AccountAction, accountKeys, balancesQuery, useAccountAction } from './api'
import { marginLevelClass } from './margin'

const ACTIONS: Record<AccountAction, { label: string; verb: string }> = {
  borrow: { label: 'Vay USDT', verb: 'Vay' },
  repay: { label: 'Trả nợ USDT', verb: 'Trả' },
  spotToCross: { label: 'Spot → Cross', verb: 'Chuyển' },
  crossToSpot: { label: 'Cross → Spot', verb: 'Chuyển' },
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-medium">{children}</span>
    </div>
  )
}

export function AccountPopover() {
  const [open, setOpen] = useState(false)
  const qc = useQueryClient()
  const { data, error, isLoading, isFetching, refetch } = useQuery({ ...balancesQuery, enabled: open })
  const [action, setAction] = useState<AccountAction | null>(null)
  const [amount, setAmount] = useState('')
  const mutation = useAccountAction()
  const amountValid = /^\d+(\.\d+)?$/.test(amount) && Number(amount) > 0

  const refresh = async () => {
    const fresh = await unwrap(api.GET('/account/balances', { params: { query: { refresh: true } } }))
    qc.setQueryData(accountKeys.balances(), fresh)
  }

  return (
    <>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button variant="ghost" size="sm" className="gap-2" aria-label="Số dư tài khoản">
            <WalletIcon />
            <span className="hidden sm:inline">Tài khoản</span>
          </Button>
        </PopoverTrigger>
        <PopoverContent align="end" className="w-80">
          <div className="mb-3 flex items-center justify-between">
            <p className="font-medium">Binance</p>
            <Button
              variant="ghost"
              size="icon"
              className="size-7"
              onClick={() => void refresh()}
              aria-label="Làm mới số dư"
              disabled={isFetching}
            >
              <RefreshCwIcon className={cn(isFetching && 'animate-spin')} />
            </Button>
          </div>
          {error ? (
            <ErrorState error={error} onRetry={() => void refetch()} />
          ) : isLoading || !data ? (
            <div className="space-y-2">
              {Array.from({ length: 6 }, (_, i) => (
                <Skeleton key={i} className="h-5 w-full" />
              ))}
            </div>
          ) : (
            <div className="space-y-2">
              <Row label="Tổng tài sản ròng">
                <Money value={data.netTotalUsdt} />
              </Row>
              <Row label="Margin level">
                <span className={cn('num', marginLevelClass(data.marginLevel))}>
                  {formatMoney(data.marginLevel)}
                </span>
              </Row>
              <Separator />
              <Row label="Spot USDT khả dụng">
                <Money value={data.spotUsdtFree} />
              </Row>
              <Row label="Cross USDT khả dụng">
                <Money value={data.crossUsdtFree} />
              </Row>
              <Row label="Nợ USDT (gốc + lãi)">
                <Money value={data.crossUsdtDebt} />
              </Row>
              <Row label="Tổng nợ quy USDT">
                <Money value={data.crossDebtUsdt} />
              </Row>
              <Separator />
              <div className="grid grid-cols-2 gap-2">
                {(Object.keys(ACTIONS) as AccountAction[]).map((a) => (
                  <Button
                    key={a}
                    variant="outline"
                    size="sm"
                    onClick={() => {
                      setAmount('')
                      setAction(a)
                    }}
                  >
                    {ACTIONS[a].label}
                  </Button>
                ))}
              </div>
              <p className="text-xs text-muted-foreground">Cập nhật {fromNow(data.fetchedAt)}</p>
            </div>
          )}
        </PopoverContent>
      </Popover>

      {action && (
        <ConfirmDialog
          open
          onOpenChange={(o) => {
            if (!o) setAction(null)
          }}
          title={ACTIONS[action].label}
          description="Lệnh được gửi thẳng lên Binance và không hoàn tác được."
          confirmLabel={`${ACTIONS[action].verb} ${amountValid ? `${formatMoney(amount)} USDT` : ''}`}
          confirmDisabled={!amountValid}
          onConfirm={async () => {
            if (!amountValid) throw new Error('invalid amount')
            await mutation.mutateAsync({ action, amount })
          }}
        >
          <div className="space-y-2">
            <Label htmlFor="account-amount">Số USDT</Label>
            <Input
              id="account-amount"
              inputMode="decimal"
              value={amount}
              onChange={(e) => setAmount(e.target.value.replace(',', '.'))}
              aria-invalid={amount !== '' && !amountValid}
            />
          </div>
        </ConfirmDialog>
      )}
    </>
  )
}
