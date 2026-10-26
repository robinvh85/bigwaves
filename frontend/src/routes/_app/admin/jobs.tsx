import { createFileRoute } from '@tanstack/react-router'

import { JobsList } from '@/features/jobs/JobsList'
import { PageHeader } from '@/shared/components/states'

export const Route = createFileRoute('/_app/admin/jobs')({
  staticData: { title: 'Jobs' },
  component: () => (
    <>
      <PageHeader
        title="Jobs"
        description="Các tác vụ chạy định kỳ. Bấm vào một job để xem lịch sử các lần chạy."
      />
      <JobsList />
    </>
  ),
})
