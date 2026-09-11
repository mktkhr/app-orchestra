import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { defineConfig, devices } from "@playwright/test";

/**
 * Browser-driven end-to-end tests.
 *
 * Playwright starts both dummy services and the built backend (`make
 * build`) serving the built frontend, all on fixed ports distinct from a
 * developer's own running processes (8080/8081/8082/5173) and from the
 * harness's own browser gates (harness/quality/browser/playwright.config.ts,
 * port 18081), then drives the platform in headless Chromium. These tests
 * prove the product as a user experiences it; the HTTP-level suite in src/
 * proves the same process without a browser.
 *
 * The platform never calls a real LLM here: no ORCHESTRA_LLM_BASE_URL is
 * set, so it falls back to the stub planner over ORCHESTRA_PLAN_FIXTURES -
 * see e2e/src/orchestration.test.ts for why that variable exists.
 */
const inventoryPort = 18083;
const attendancePort = 18084;
const platformPort = 18080;

const planFixtures = [
  { query: "在庫の一覧を見せて", service: "inventory", operationId: "ListInventoryItems" },
];

// A file in its own temporary directory, per docs/specs/workspaces.md
// section 6: this suite must not see workspaces another suite wrote, and
// ORCHESTRA_DB_PATH has no default (internal/infra/config.ErrMissingDBPath)
// for the platform to fall back to instead.
const dbPath = join(mkdtempSync(join(tmpdir(), "orchestra-e2e-browser-")), "workspaces.db");

export default defineConfig({
  testDir: "./browser",
  fullyParallel: true,
  forbidOnly: true,
  retries: 0,
  reporter: [["list"]],
  timeout: 30_000,
  use: {
    baseURL: `http://127.0.0.1:${platformPort}`,
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: [
    {
      command: "../services/inventory/bin/api",
      url: `http://127.0.0.1:${inventoryPort}/openapi.yaml`,
      reuseExistingServer: false,
      timeout: 15_000,
      env: { ORCHESTRA_PORT: String(inventoryPort) },
    },
    {
      command: "../services/attendance/bin/api",
      url: `http://127.0.0.1:${attendancePort}/openapi.yaml`,
      reuseExistingServer: false,
      timeout: 15_000,
      env: { ORCHESTRA_PORT: String(attendancePort) },
    },
    {
      command: "../services/platform/bin/api",
      url: `http://127.0.0.1:${platformPort}/api/health`,
      reuseExistingServer: false,
      timeout: 15_000,
      env: {
        ORCHESTRA_PORT: String(platformPort),
        ORCHESTRA_STATIC_DIR: "../web/dist",
        ORCHESTRA_SERVICES:
          `inventory=http://127.0.0.1:${inventoryPort},` +
          `attendance=http://127.0.0.1:${attendancePort}`,
        ORCHESTRA_PLAN_FIXTURES: JSON.stringify(planFixtures),
        ORCHESTRA_DB_PATH: dbPath,
      },
    },
  ],
});
