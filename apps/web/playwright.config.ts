import { defineConfig } from '@playwright/test';

// In Docker (make e2e) a small TCP proxy makes localhost:3000/3001/8090 point at web/admin/mockext,
// so the browser uses exactly the URLs a presenter uses on the host. On the host the ports are taken
// and Playwright reuses the running stack.
export default defineConfig({
  testDir: './e2e',
  timeout: 120_000,
  use: { baseURL: process.env.BASE_URL ?? 'http://localhost:3000', locale: 'th-TH', trace: 'retain-on-failure' },
  reporter: [['list']],
  webServer: { command: 'node ../../deploy/e2e/localhost-proxy.mjs', port: 8090, reuseExistingServer: true },
});
