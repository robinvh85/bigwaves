import { createFileRoute } from '@tanstack/react-router'

import { ComingSoon } from '@/shared/components/coming-soon'

export const Route = createFileRoute('/_app/ema1d/pairs')({
  staticData: { title: 'Rổ coin' },
  component: () => <ComingSoon title="Rổ coin" phase="F2" />,
})
