import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./tests",
  expect: { timeout: 15000 },
  use: {
    baseURL: "http://127.0.0.1:15172",
    headless: true,
    channel: process.env.PLAYWRIGHT_CHANNEL || undefined,
  },
  webServer: {
    command: "npm run preview -- --port 15172",
    url: "http://127.0.0.1:15172",
    reuseExistingServer: false,
  },
  reporter: "list",
});
