import { createFileRoute } from '@tanstack/react-router'

import { ComingSoon } from '@/shared/components/coming-soon'

export const Route = createFileRoute('/_app/ema1d/todos')({
  staticData: { title: 'Hàng đợi' },
  component: () => <ComingSoon title="Hàng đợi" phase="F2" />,
})
