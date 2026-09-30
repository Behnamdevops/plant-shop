import { defineConfig } from '@playwright/test'
export default defineConfig({
  testDir: './e2e',
  workers: 1,
  use: {
    baseURL: process.env.E2E_BASE_URL || 'http://localhost:4173',
    headless: true,
    launchOptions: process.env.CHROMIUM_PATH
      ? { executablePath: process.env.CHROMIUM_PATH, args: ['--no-sandbox'] }
      : undefined,
  },
  reporter: 'list',
})
