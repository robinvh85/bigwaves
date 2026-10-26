import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'

import { ConfirmDialog } from '@/shared/components/confirm-dialog'
import { Label } from '@/shared/ui/label'
import { Switch } from '@/shared/ui/switch'

import { configQuery, type Ema1dConfig, useSaveConfig } from '../api'

type Key = 'enabled' | 'pauseOpen' | 'dailyCheckEnabled'

const LABELS: Record<Key, { label: string; on: string; off: string }> = {
  enabled: {
    label: 'Thực thi lệnh',
    on: 'Hàng đợi sẽ bắt đầu đặt lệnh thật lên Binance (nếu TRADING_ENABLED đang bật).',
    off: 'Hàng đợi dừng lại: todo vẫn được tạo nhưng không được thực thi.',
  },
  pauseOpen: {
    label: 'Tạm dừng mở mới',
    on: 'Sync, lệnh tay và hàng đợi sẽ không mở lệnh mới. Lệnh đóng vẫn chạy.',
    off: 'Cho phép mở lệnh mới trở lại.',
  },
  dailyCheckEnabled: {
    label: 'Daily check 07:03',
    on: 'Mỗi sáng 07:03 VN: tải nến, tính tín hiệu, sync và gửi báo cáo Discord.',
    off: 'Không tự chạy daily check nữa.',
  },
}

/** The three strategy switches, each behind a confirmation that says what will happen. */
export function StrategySwitches() {
  const { data } = useQuery(configQuery)
  const save = useSaveConfig()
  const [pending, setPending] = useState<{ key: Key; value: boolean } | null>(null)
  if (!data) return null
  const cfg = data.config

  return (
    <div className="flex flex-wrap items-center gap-x-5 gap-y-2">
      {(Object.keys(LABELS) as Key[]).map((key) => (
        <div key={key} className="flex items-center gap-2">
          <Switch
            id={`sw-${key}`}
            checked={cfg[key]}
            onCheckedChange={(value) => setPending({ key, value })}
            disabled={save.isPending}
          />
          <Label htmlFor={`sw-${key}`} className="text-sm">
            {LABELS[key].label}
          </Label>
        </div>
      ))}
      {pending && (
        <ConfirmDialog
          open
          onOpenChange={(o) => {
            if (!o) setPending(null)
          }}
          title={`${pending.value ? 'Bật' : 'Tắt'} "${LABELS[pending.key].label}"?`}
          description={pending.value ? LABELS[pending.key].on : LABELS[pending.key].off}
          destructive={pending.key === 'enabled' && pending.value}
          confirmLabel={pending.value ? 'Bật' : 'Tắt'}
          onConfirm={async () => {
            const next: Ema1dConfig = { ...cfg, [pending.key]: pending.value }
            await save.mutateAsync({ version: data.version, config: next })
          }}
        />
      )}
    </div>
  )
}
