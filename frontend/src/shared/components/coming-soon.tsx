import { ConstructionIcon } from 'lucide-react'

import { EmptyState, PageHeader } from '@/shared/components/states'

/** Temporary page body for routes whose feature is not built yet. */
export function ComingSoon({ title, phase }: { title: string; phase: string }) {
  return (
    <>
      <PageHeader title={title} />
      <EmptyState
        icon={ConstructionIcon}
        title="Đang xây dựng"
        description={`Màn hình này thuộc pha ${phase}.`}
      />
    </>
  )
}
