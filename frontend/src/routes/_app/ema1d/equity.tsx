import { useQuery } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { z } from 'zod'

import { equityQuery } from '@/features/ema1d/api'
import { DailyChecks } from '@/features/ema1d/components/DailyChecks'
import { EquityChart } from '@/features/ema1d/components/EquityChart'
import { SignalsTable } from '@/features/ema1d/components/SignalsTable'
import { EmptyState, ErrorState, PageHeader, SkeletonRows } from '@/shared/components/states'
import { Card, CardContent, CardHeader, CardTitle } from '@/shared/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/shared/ui/tabs'
import { ToggleGroup, ToggleGroupItem } from '@/shared/ui/toggle-group'

const search = z.object({
  tab: z.enum(['equity', 'reports', 'signals']).optional(),
  days: z.number().int().optional(),
})

export const Route = createFileRoute('/_app/ema1d/equity')({
  staticData: { title: 'Equity & reports' },
  validateSearch: search,
  component: EquityPage,
})

function EquityPage() {
  const { tab = 'equity', days = 180 } = Route.useSearch()
  const navigate = useNavigate({ from: Route.fullPath })
  const set = (patch: z.infer<typeof search>) =>
    void navigate({ search: (p) => ({ ...p, ...patch }), replace: true })
  const { data, isLoading, error, refetch } = useQuery(equityQuery(days))

  return (
    <>
      <PageHeader
        title="Equity & reports"
        description="Daily equity curve, daily check reports and the current signal table."
      />
      <Tabs value={tab} onValueChange={(v) => set({ tab: v as 'equity' })}>
        <TabsList>
          <TabsTrigger value="equity">Equity curve</TabsTrigger>
          <TabsTrigger value="reports">Daily check</TabsTrigger>
          <TabsTrigger value="signals">Signals</TabsTrigger>
        </TabsList>
        <TabsContent value="equity">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle className="text-base">Strategy capital (incl. unrealized PnL)</CardTitle>
              <ToggleGroup
                type="single"
                size="sm"
                variant="outline"
                value={String(days)}
                onValueChange={(v) => v && set({ days: Number(v) })}
              >
                <ToggleGroupItem value="30">30N</ToggleGroupItem>
                <ToggleGroupItem value="90">90N</ToggleGroupItem>
                <ToggleGroupItem value="180">180N</ToggleGroupItem>
                <ToggleGroupItem value="365">1N</ToggleGroupItem>
              </ToggleGroup>
            </CardHeader>
            <CardContent>
              {error ? (
                <ErrorState error={error} onRetry={() => void refetch()} />
              ) : isLoading || !data ? (
                <SkeletonRows rows={6} />
              ) : data.length < 2 ? (
                <EmptyState
                  title="Not enough data yet"
                  description="An equity snapshot is taken every day at 07:05 ICT."
                />
              ) : (
                <EquityChart points={data} />
              )}
            </CardContent>
          </Card>
        </TabsContent>
        <TabsContent value="reports">
          <DailyChecks />
        </TabsContent>
        <TabsContent value="signals">
          <SignalsTable />
        </TabsContent>
      </Tabs>
    </>
  )
}
