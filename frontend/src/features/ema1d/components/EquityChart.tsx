import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from 'recharts'

import { formatMoney } from '@/shared/lib/format'
import { dayjs } from '@/shared/lib/date'
import { type ChartConfig, ChartContainer, ChartTooltip, ChartTooltipContent } from '@/shared/ui/chart'

import type { components } from '@/shared/api/schema'

type Point = components['schemas']['Ema1dEquityPoint']

const config = { equity: { label: 'Vốn (USDT)', color: 'var(--chart-2)' } } satisfies ChartConfig

export function EquityChart({ points }: { points: Point[] }) {
  const data = points.map((p) => ({ date: p.date, equity: Number(p.equityUsdt) }))
  const values = data.map((d) => d.equity)
  const min = Math.min(...values)
  const max = Math.max(...values)
  const pad = Math.max((max - min) * 0.1, 1)
  return (
    <ChartContainer config={config} className="aspect-auto h-72 w-full">
      <AreaChart data={data} margin={{ left: 8, right: 8, top: 8 }}>
        <defs>
          <linearGradient id="fillEquity" x1="0" y1="0" x2="0" y2="1">
            <stop offset="5%" stopColor="var(--color-equity)" stopOpacity={0.35} />
            <stop offset="95%" stopColor="var(--color-equity)" stopOpacity={0.02} />
          </linearGradient>
        </defs>
        <CartesianGrid vertical={false} />
        <XAxis
          dataKey="date"
          tickLine={false}
          axisLine={false}
          minTickGap={32}
          tickFormatter={(v: string) => dayjs(v).format('DD/MM')}
        />
        <YAxis
          domain={[min - pad, max + pad]}
          tickLine={false}
          axisLine={false}
          width={64}
          tickFormatter={(v: number) => formatMoney(v, 0)}
        />
        <ChartTooltip
          content={
            <ChartTooltipContent
              labelFormatter={(v) => dayjs(String(v)).format('DD/MM/YYYY')}
              formatter={(v) => `${formatMoney(Number(v))} USDT`}
            />
          }
        />
        <Area
          dataKey="equity"
          type="monotone"
          stroke="var(--color-equity)"
          fill="url(#fillEquity)"
          strokeWidth={2}
          isAnimationActive={false}
        />
      </AreaChart>
    </ChartContainer>
  )
}
