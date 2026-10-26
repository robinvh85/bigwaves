import { defineConfig, devices } from '@playwright/test'

// Smoke tests against the real binary (frontend embedded), on a throwaway database, signed in
// through the dev login. Binance is pointed at a closed port: no network call leaves the machine.
const port = 18099
const db = `${process.env.TMPDIR ?? '/tmp'}/bigwaves-e2e-${Date.now()}.db`

export default defineConfig({
  testDir: 'e2e',
  timeout: 30_000,
  fullyParallel: false,
  reporter: [['list']],
  use: { baseURL: `http://127.0.0.1:${port}`, trace: 'retain-on-failure' },
  projects: [
    { name: 'desktop', use: { ...devices['Desktop Chrome'], viewport: { width: 1440, height: 900 } } },
    { name: 'mobile', use: { ...devices['Pixel 7'] } },
  ],
  webServer: {
    command: `cd .. && make build-fe >/dev/null && cd backend && go build -o bin/bigwaves-e2e ./cmd/bigwaves && ./bin/bigwaves-e2e`,
    url: `http://127.0.0.1:${port}/healthz`,
    timeout: 180_000,
    reuseExistingServer: false,
    env: {
      HTTP_ADDR: `127.0.0.1:${port}`,
      DB_PATH: db,
      APP_URL: `http://127.0.0.1:${port}`,
      DEV_USER_EMAIL: 'e2e@example.com',
      SCHEDULER_ENABLED: 'false',
      BINANCE_BASE_URL: 'http://127.0.0.1:9',
      LOG_LEVEL: 'warn',
    },
  },
})
