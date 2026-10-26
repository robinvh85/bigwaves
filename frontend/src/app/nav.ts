import {
  ActivityIcon,
  BotIcon,
  CandlestickChartIcon,
  CoinsIcon,
  HistoryIcon,
  LayoutDashboardIcon,
  LineChartIcon,
  ListChecksIcon,
  type LucideIcon,
  SettingsIcon,
  SlidersHorizontalIcon,
  TrendingUpIcon,
} from 'lucide-react'

export type NavItem = { title: string; to: string; icon: LucideIcon; exact?: boolean }
export type NavGroup = { title?: string; items: NavItem[] }

/** Sidebar structure; mirrors the information architecture in the plan (02-frontend-react §4). */
export const NAV: NavGroup[] = [
  {
    items: [{ title: 'Tổng quan', to: '/', icon: LayoutDashboardIcon, exact: true }],
  },
  {
    title: 'EMA1D',
    items: [
      { title: 'Bảng điều khiển', to: '/ema1d', icon: CandlestickChartIcon, exact: true },
      { title: 'Rổ coin', to: '/ema1d/pairs', icon: CoinsIcon },
      { title: 'Hàng đợi', to: '/ema1d/todos', icon: ListChecksIcon },
      { title: 'Lịch sử lệnh', to: '/ema1d/trades', icon: HistoryIcon },
      { title: 'Equity', to: '/ema1d/equity', icon: LineChartIcon },
      { title: 'Cấu hình', to: '/ema1d/config', icon: SlidersHorizontalIcon },
    ],
  },
  {
    title: 'DCA',
    items: [
      { title: 'Danh sách', to: '/dca', icon: TrendingUpIcon, exact: true },
      { title: 'Cấu hình', to: '/dca/config', icon: SlidersHorizontalIcon },
    ],
  },
  {
    title: 'Quản trị',
    items: [
      { title: 'Pairs', to: '/admin/pairs', icon: BotIcon },
      { title: 'Jobs', to: '/admin/jobs', icon: ActivityIcon },
    ],
  },
]

export const SETTINGS_ICON = SettingsIcon
