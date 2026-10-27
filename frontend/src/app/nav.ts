import {
  ActivityIcon,
  BellRingIcon,
  BotIcon,
  CandlestickChartIcon,
  CoinsIcon,
  HistoryIcon,
  LayoutDashboardIcon,
  LineChartIcon,
  ListChecksIcon,
  type LucideIcon,
  RadarIcon,
  SettingsIcon,
  SlidersHorizontalIcon,
  TrendingUpIcon,
} from 'lucide-react'

export type NavItem = { title: string; to: string; icon: LucideIcon; exact?: boolean }
export type NavGroup = { title?: string; items: NavItem[] }

/** Sidebar structure; mirrors the information architecture in the plan (02-frontend-react §4). */
export const NAV: NavGroup[] = [
  {
    items: [{ title: 'Overview', to: '/', icon: LayoutDashboardIcon, exact: true }],
  },
  {
    title: 'EMA1D',
    items: [
      { title: 'Dashboard', to: '/ema1d', icon: CandlestickChartIcon, exact: true },
      { title: 'Basket', to: '/ema1d/pairs', icon: CoinsIcon },
      { title: 'Queue', to: '/ema1d/todos', icon: ListChecksIcon },
      { title: 'Trade history', to: '/ema1d/trades', icon: HistoryIcon },
      { title: 'Equity & reports', to: '/ema1d/equity', icon: LineChartIcon },
      { title: 'Settings', to: '/ema1d/config', icon: SlidersHorizontalIcon },
    ],
  },
  {
    title: 'DCA',
    items: [
      { title: 'Ladders', to: '/dca', icon: TrendingUpIcon, exact: true },
      { title: 'Settings', to: '/dca/config', icon: SlidersHorizontalIcon },
    ],
  },
  {
    title: 'EMA Cross',
    items: [
      { title: 'Watchlist', to: '/ema-cross', icon: RadarIcon, exact: true },
      { title: 'Alerts', to: '/ema-cross/alerts', icon: BellRingIcon },
      { title: 'Settings', to: '/ema-cross/config', icon: SlidersHorizontalIcon },
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

export const SETTINGS_ICON = SettingsIcon
