import AxeBuilder from '@axe-core/playwright'
import { expect, type Page, test } from '@playwright/test'

const pages = [
  { path: '/', heading: 'Overview' },
  { path: '/ema1d', heading: 'EMA1D-EXIT' },
  { path: '/ema1d/pairs', heading: 'Basket' },
  { path: '/ema1d/todos', heading: 'Order queue' },
  { path: '/ema1d/trades', heading: 'Trade history' },
  { path: '/ema1d/equity', heading: 'Equity & reports' },
  { path: '/ema1d/config', heading: 'EMA1D settings' },
  { path: '/dca', heading: 'DCA' },
  { path: '/dca/config', heading: 'DCA settings' },
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
  await expect(page.getByRole('button', { name: 'Sign in with Auth0' })).toBeVisible()
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
  await expect(page.getByText('Read-only')).toBeVisible()
})

test('the sidebar navigates', async ({ page, isMobile }) => {
  await page.goto('/')
  if (isMobile) await page.getByRole('button', { name: /Toggle Sidebar/i }).click()
  await page.getByRole('link', { name: 'Basket' }).click()
  await expect(page.getByRole('heading', { level: 1, name: 'Basket' })).toBeVisible()
})

test('the EMA1D dashboard has no strategy switches', async ({ page }) => {
  await page.goto('/ema1d')
  await expect(page.getByRole('heading', { level: 1, name: 'EMA1D-EXIT' })).toBeVisible()
  await expect(page.getByRole('switch')).toHaveCount(0)
})

test('numeric inputs only take digits and one decimal point', async ({ page }) => {
  await page.goto('/ema1d')
  await page.getByRole('button', { name: 'Manual order' }).click()
  const price = page.getByLabel('Reference price (optional)')
  await price.pressSequentially('12a.3b4,5')
  await expect(price).toHaveValue('12.345')
  await price.fill('abc1,5x')
  await expect(price).toHaveValue('1.5')
})

test('the manual order coin picker can be searched', async ({ page }) => {
  await page.goto('/ema1d')
  await page.getByRole('button', { name: 'Manual order' }).click()
  await page.getByRole('combobox', { name: 'Coin' }).click()
  const search = page.getByPlaceholder('Search coin…')
  await expect(search).toBeFocused()
  await search.fill('zzzz')
  await expect(page.getByText('No coin found')).toBeVisible()
})
