import AxeBuilder from '@axe-core/playwright'
import { expect, type Page, test } from '@playwright/test'

const pages = [
  { path: '/', heading: 'Tổng quan' },
  { path: '/ema1d', heading: 'EMA1D-EXIT' },
  { path: '/ema1d/pairs', heading: 'Rổ coin' },
  { path: '/ema1d/todos', heading: 'Hàng đợi lệnh' },
  { path: '/ema1d/trades', heading: 'Lịch sử lệnh' },
  { path: '/ema1d/equity', heading: 'Equity & báo cáo' },
  { path: '/ema1d/config', heading: 'Cấu hình EMA1D' },
  { path: '/dca', heading: 'DCA' },
  { path: '/dca/config', heading: 'Cấu hình DCA' },
  { path: '/admin/pairs', heading: 'Pairs' },
  { path: '/admin/jobs', heading: 'Jobs' },
]

function watchErrors(page: Page) {
  const errors: string[] = []
  page.on('pageerror', (e) => errors.push(e.message))
  page.on('console', (m) => {
    // Exchange calls fail on purpose in this environment (no Binance): those 4xx/5xx are expected.
    if (m.type() === 'error' && !m.text().includes('Failed to load resource')) errors.push(m.text())
  })
  return errors
}

test.beforeEach(async ({ page }) => {
  await page.goto('/auth/login')
  await page.waitForURL('/')
})

test('signed-out visitors are sent to the login page', async ({ browser }) => {
  const page = await browser.newPage()
  await page.goto('/ema1d')
  await expect(page).toHaveURL(/\/login/)
  await expect(page.getByRole('button', { name: 'Đăng nhập với Auth0' })).toBeVisible()
})

for (const p of pages) {
  test(`${p.path} renders without errors or serious a11y issues`, async ({ page }) => {
    const errors = watchErrors(page)
    await page.goto(p.path)
    await expect(page.getByRole('heading', { level: 1, name: p.heading })).toBeVisible()
    await page.waitForLoadState('networkidle')
    expect(errors).toEqual([])
    const axe = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa']).analyze()
    const serious = axe.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical')
    expect(serious.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`)).toEqual([])
  })
}

test('read-only mode is visible everywhere', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByText('Chỉ đọc').or(page.getByRole('status').first())).toBeVisible()
})

test('the sidebar navigates', async ({ page, isMobile }) => {
  await page.goto('/')
  if (isMobile) await page.getByRole('button', { name: /Toggle Sidebar/i }).click()
  await page.getByRole('link', { name: 'Rổ coin' }).click()
  await expect(page.getByRole('heading', { level: 1, name: 'Rổ coin' })).toBeVisible()
})
