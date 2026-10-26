import { createFileRoute } from '@tanstack/react-router'

import { ComingSoon } from '@/shared/components/coming-soon'

export const Route = createFileRoute('/_app/admin/pairs')({
  staticData: { title: 'Pairs' },
  component: () => <ComingSoon title="Pairs" phase="F1" />,
})
