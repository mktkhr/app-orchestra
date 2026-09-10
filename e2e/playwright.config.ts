import { defineConfig, devices } from "@playwright/test";

/**
 * Browser-driven end-to-end tests.
 *
 * Playwright starts the built backend (`make build`) serving the built
 * frontend on a fixed port and drives it in headless Chromium. These tests
 * prove the product as a user experiences it; the HTTP-level suite in src/
 * proves the same process without a browser.
 */
const port = 18080;

export default defineConfig({
  testDir: "./browser",
  fullyParallel: true,
  forbidOnly: true,
  retries: 0,
  reporter: [["list"]],
  timeout: 30_000,
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command: "../services/platform/bin/api",
    url: `http://127.0.0.1:${port}/api/health`,
    reuseExistingServer: false,
    timeout: 15_000,
    env: {
      ORCHESTRA_PORT: String(port),
      ORCHESTRA_STATIC_DIR: "../web/dist",
    },
  },
});
