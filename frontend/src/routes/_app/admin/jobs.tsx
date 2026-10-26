import { createFileRoute } from '@tanstack/react-router'

import { ComingSoon } from '@/shared/components/coming-soon'

export const Route = createFileRoute('/_app/admin/jobs')({
  staticData: { title: 'Jobs' },
  component: () => <ComingSoon title="Jobs" phase="F1" />,
})
