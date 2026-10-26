import { createFileRoute } from '@tanstack/react-router'

import { ComingSoon } from '@/shared/components/coming-soon'

export const Route = createFileRoute('/_app/')({
  staticData: { title: 'Tổng quan' },
  component: () => <ComingSoon title="Tổng quan" phase="F2" />,
})
