import { createFileRoute } from '@tanstack/react-router'

import { ComingSoon } from '@/shared/components/coming-soon'

export const Route = createFileRoute('/_app/ema1d/')({
  staticData: { title: 'Bảng điều khiển' },
  component: () => <ComingSoon title="Bảng điều khiển" phase="F2" />,
})
