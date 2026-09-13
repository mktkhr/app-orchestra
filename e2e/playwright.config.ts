import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { defineConfig, devices } from "@playwright/test";

import { ADMIN_PASSWORD } from "./browser/helpers/constants";

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
 *
 * Every route but GET /api/health and POST /api/session now answers 401
 * without a session (docs/specs/auth.md, AC-A-102), so every spec signs in
 * first through browser/helpers/auth.ts's signInAsAdmin
 * (docs/plans/auth.md, Task 6, Step 1).
 */
const inventoryPort = 18083;
const attendancePort = 18084;
const platformPort = 18080;

const planFixtures = [
  { query: "在庫の一覧を見せて", service: "inventory", operationId: "ListInventoryItems" },
  // Answered only when it follows a turn that resolved to inventory - the
  // stub's Key now carries the conversation too (docs/plans/context.md,
  // Task 4), so `context.spec.ts` can drive a follow-up question that names
  // no service and still land back on the same one.
  {
    query: "検品保留のものだけ見せて",
    turns: [{ service: "inventory", operationId: "ListInventoryItems" }],
    service: "inventory",
    operationId: "ListInventoryItems",
  },
  // proposing.spec.ts: a propose_panel-shaped fixture (docs/plans/proposing.md,
  // Task 2), the same query/response WorkspacePage.test.tsx already exercises
  // at the component level.
  {
    query: "在庫をステータス別に棒グラフで置いて",
    propose: true,
    service: "inventory",
    operationId: "ListInventoryItems",
    component: "chart",
    title: "ステータス別の在庫",
    chart: { category: "status", value: "count", kind: "bar" },
  },
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
        // ORCHESTRA_ADMIN_PASSWORD has no default either
        // (internal/infra/config.ErrMissingAdminPassword). Kept in one
        // place (browser/helpers/constants.ts) since every spec now signs
        // in as this account (docs/plans/auth.md, Task 6).
        ORCHESTRA_ADMIN_PASSWORD: ADMIN_PASSWORD,
        // This suite runs over plain HTTP (127.0.0.1, no TLS): a Secure
        // cookie is never stored by the browser, and signing in below
        // would appear to work while the session never actually carried
        // (internal/infra/config.Config.SecureCookie).
        ORCHESTRA_SECURE_COOKIE: "false",
      },
    },
  ],
});
