import { defineConfig } from '@playwright/test';

// make app-e2e: runs in the existing `playwright` compose service; app-proxy.mjs maps localhost:3002/8000/8090
// to app/gateway/mockext. On the host those ports are already served, so Playwright reuses them.
export default defineConfig({
  testDir: '.',
  timeout: 120_000,
  use: {
    baseURL: process.env.APP_URL ?? 'http://localhost:3002',
    viewport: { width: 400, height: 860 },
    locale: 'th-TH',
    trace: 'retain-on-failure',
  },
  reporter: [['list']],
  webServer: { command: 'node ../../../deploy/e2e/app-proxy.mjs', port: 3002, reuseExistingServer: true },
});
