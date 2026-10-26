import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'

import { dcaConfigQuery, dcaHistoryQuery, useSaveDcaConfig } from '@/features/dca/api'
import { ConfirmDialog } from '@/shared/components/confirm-dialog'
import { ErrorState, PageHeader, SkeletonRows } from '@/shared/components/states'
import { formatDateTime } from '@/shared/lib/date'
import { Button } from '@/shared/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card'
import { Input } from '@/shared/ui/input'
import { Label } from '@/shared/ui/label'
import { Switch } from '@/shared/ui/switch'

export const Route = createFileRoute('/_app/dca/config')({
  staticData: { title: 'Cấu hình' },
  component: DcaConfigPage,
})

function DcaConfigPage() {
  const { data, error, refetch } = useQuery(dcaConfigQuery)
  const { data: history } = useQuery(dcaHistoryQuery)
  const save = useSaveDcaConfig()
  const [cap, setCap] = useState<string | null>(null)
  const [pending, setPending] = useState<{ enabled: boolean; maxCapitalUsdt: string } | null>(null)

  if (error) return <ErrorState error={error} onRetry={() => void refetch()} />
  if (!data) return <SkeletonRows rows={4} />
  const c = data.config
  const capValue = cap ?? c.maxCapitalUsdt
  const capValid = /^\d+(\.\d+)?$/.test(capValue)

  return (
    <>
      <PageHeader title="Cấu hình DCA" />
      <div className="max-w-2xl space-y-4">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Chạy tự động</CardTitle>
            <CardDescription>Tắt: các ladder được giữ nguyên nhưng không lệnh nào được khớp.</CardDescription>
          </CardHeader>
          <CardContent className="flex items-center gap-3">
            <Switch
              id="dca-enabled"
              checked={c.enabled}
              onCheckedChange={(v) => setPending({ ...c, enabled: v })}
            />
            <Label htmlFor="dca-enabled">{c.enabled ? 'Đang bật' : 'Đang tắt'}</Label>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Giới hạn vốn</CardTitle>
            <CardDescription>
              Tổng USDT đang nằm trong các nấc đã mua, trên mọi ladder. Lệnh mua làm vượt giới hạn sẽ bị từ
              chối. 0 = không giới hạn.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex items-end gap-2">
            <div className="grid flex-1 gap-2">
              <Label htmlFor="dca-cap">USDT</Label>
              <Input
                id="dca-cap"
                inputMode="decimal"
                value={capValue}
                onChange={(e) => setCap(e.target.value)}
                aria-invalid={!capValid}
              />
            </div>
            <Button
              disabled={!capValid || capValue === c.maxCapitalUsdt}
              onClick={() => setPending({ ...c, maxCapitalUsdt: capValue })}
            >
              Lưu
            </Button>
          </CardContent>
        </Card>
        {history && history.length > 0 && (
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Lịch sử thay đổi</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              {history.map((h) => (
                <p key={h.id}>
                  <span className="text-muted-foreground">
                    {formatDateTime(h.createdAt)} · {h.userEmail}:
                  </span>{' '}
                  {JSON.stringify(h.before)} → {JSON.stringify(h.after)}
                </p>
              ))}
            </CardContent>
          </Card>
        )}
      </div>
      {pending && (
        <ConfirmDialog
          open
          onOpenChange={(o) => !o && setPending(null)}
          title="Lưu cấu hình DCA?"
          description={`Chạy tự động: ${pending.enabled ? 'bật' : 'tắt'} · giới hạn vốn: ${pending.maxCapitalUsdt === '0' ? 'không' : `${pending.maxCapitalUsdt} USDT`}`}
          onConfirm={async () => {
            await save.mutateAsync({ version: data.version, config: pending })
            setCap(null)
          }}
        />
      )}
    </>
  )
}
