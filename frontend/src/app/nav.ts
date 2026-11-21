import {
  ActivityIcon,
  BotIcon,
  CandlestickChartIcon,
  LayoutDashboardIcon,
  type LucideIcon,
  RadarIcon,
  SettingsIcon,
  TargetIcon,
  TrendingUpIcon,
} from 'lucide-react'

import type { SectionTab } from '@/shared/components/section-tabs'

export type NavItem = { title: string; to: string; icon: LucideIcon; exact?: boolean }
export type NavGroup = { title?: string; items: NavItem[] }

/**
 * Sidebar: one entry per strategy. The screens of a strategy are tabs at the top of its pages
 * (SECTION_TABS), not sidebar entries.
 */
export const NAV: NavGroup[] = [
  {
    items: [{ title: 'Overview', to: '/', icon: LayoutDashboardIcon, exact: true }],
  },
  {
    title: 'Strategies',
    items: [
      { title: 'EMA1D', to: '/ema1d', icon: CandlestickChartIcon },
      { title: 'DCA', to: '/dca', icon: TrendingUpIcon },
      { title: 'EMA Cross', to: '/ema-cross', icon: RadarIcon },
      { title: 'EMA Settings', to: '/ema-settings', icon: TargetIcon },
    ],
  },
  {
    title: 'Admin',
    items: [
      { title: 'Pairs', to: '/admin/pairs', icon: BotIcon },
      { title: 'Jobs', to: '/admin/jobs', icon: ActivityIcon },
    ],
  },
]

export const SECTION_TABS = {
  ema1d: [
    { title: 'Dashboard', to: '/ema1d', exact: true },
    { title: 'Basket', to: '/ema1d/pairs' },
    { title: 'Queue', to: '/ema1d/todos' },
    { title: 'Trades', to: '/ema1d/trades' },
    { title: 'Equity & reports', to: '/ema1d/equity' },
    { title: 'Settings', to: '/ema1d/config' },
  ],
  dca: [
    { title: 'Ladders', to: '/dca', exact: true },
    { title: 'Settings', to: '/dca/config' },
  ],
  emaCross: [
    { title: 'Watchlist', to: '/ema-cross', exact: true },
    { title: 'Alerts', to: '/ema-cross/alerts' },
    { title: 'Settings', to: '/ema-cross/config' },
  ],
  emaSettings: [
    { title: 'Campaigns', to: '/ema-settings', exact: true },
    { title: 'Settings', to: '/ema-settings/config' },
  ],
} satisfies Record<string, SectionTab[]>

export const SETTINGS_ICON = SettingsIcon
