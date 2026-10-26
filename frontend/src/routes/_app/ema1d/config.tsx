import { createFileRoute } from '@tanstack/react-router'

import { ComingSoon } from '@/shared/components/coming-soon'

export const Route = createFileRoute('/_app/ema1d/config')({
  staticData: { title: 'Cấu hình' },
  component: () => <ComingSoon title="Cấu hình" phase="F2" />,
})
