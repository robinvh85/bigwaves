import { createFileRoute } from '@tanstack/react-router'

import { ComingSoon } from '@/shared/components/coming-soon'

export const Route = createFileRoute('/_app/ema1d/equity')({
  staticData: { title: 'Equity' },
  component: () => <ComingSoon title="Equity" phase="F2" />,
})
