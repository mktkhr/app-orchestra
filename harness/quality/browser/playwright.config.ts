import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { defineConfig, devices } from "@playwright/test";

/**
 * Harness-owned browser gates.
 *
 * Separate from e2e/playwright.config.ts on purpose: those specs
 * are the product's acceptance criteria and belong to the agent, these are
 * quality gates and live under harness/quality/ where the agent cannot reach them.
 *
 * Run through `make guard-a11y`, which is part of `make check`.
 */
const port = 18081;

// The platform requires a database and will not start without one being
// named. These gates measure a screen rather than what is on it, so the file
// is a scratch one outside the repository - a fresh directory each run, left
// for the operating system to sweep up.
const databasePath = join(mkdtempSync(join(tmpdir(), "orchestra-guard-")), "orchestra.db");

export default defineConfig({
  testDir: ".",
  // Outside harness/quality/ on purpose. The default is <configDir>/test-results, which
  // would put traces inside a protected path: ignoring them trips the ignore
  // guard, and not ignoring them leaves build output in the quality baseline,
  // with the agent unable to fix either since it cannot write here.
  outputDir: "../../../test-results/quality-browser",
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
    command: "../../../services/platform/bin/api",
    url: `http://127.0.0.1:${port}/api/health`,
    reuseExistingServer: false,
    timeout: 15_000,
    env: {
      ORCHESTRA_PORT: String(port),
      ORCHESTRA_STATIC_DIR: "../../../web/dist",
      ORCHESTRA_DB_PATH: databasePath,
      // ORCHESTRA_ADMIN_PASSWORD has no default either
      // (internal/infra/config.ErrMissingAdminPassword) - these gates
      // measure a screen rather than what is on it, so a fixed value is
      // fine.
      ORCHESTRA_ADMIN_PASSWORD: "guard-admin-password",
    },
  },
});
