import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  timeout: 120_000,
  use: { baseURL: process.env.BASE_URL ?? 'http://localhost:3000', locale: 'th-TH', trace: 'retain-on-failure' },
  reporter: [['list']],
});
