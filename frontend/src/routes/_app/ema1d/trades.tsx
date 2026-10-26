import { createFileRoute } from '@tanstack/react-router'

import { ComingSoon } from '@/shared/components/coming-soon'

export const Route = createFileRoute('/_app/ema1d/trades')({
  staticData: { title: 'Lịch sử lệnh' },
  component: () => <ComingSoon title="Lịch sử lệnh" phase="F2" />,
})
