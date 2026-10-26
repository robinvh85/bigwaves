import { createFileRoute } from '@tanstack/react-router'

import { ComingSoon } from '@/shared/components/coming-soon'

export const Route = createFileRoute('/_app/dca/')({
  staticData: { title: 'Danh sách' },
  component: () => <ComingSoon title="Danh sách" phase="F3" />,
})
