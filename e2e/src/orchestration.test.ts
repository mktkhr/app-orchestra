import { type ChildProcess, spawn } from "node:child_process";
import { mkdtempSync } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { setTimeout as delay } from "node:timers/promises";

import { afterAll, beforeAll, describe, expect, it } from "vite-plus/test";

/**
 * Process-level end to end suite (AC-E-101).
 *
 * Starts both dummy services and the platform from their built binaries
 * (`make build`), asks a question over real TCP against the platform's
 * `/api/plan`, and asserts the answer the dummy inventory service produced.
 * `e2e/browser/chat.spec.ts` proves the same product through a browser; this
 * file proves the same process without one.
 *
 * The platform never calls a real LLM here: no `ORCHESTRA_LLM_BASE_URL` is
 * set, so it falls back to the stub planner (`pkg/app.newPlanner`). The
 * stub's table is empty by default (`defaultPlanFixtures` was removed - see
 * DECISIONS.md), so the question this test asks is fed to the platform
 * through `ORCHESTRA_PLAN_FIXTURES`, a JSON-encoded array of
 * `config.PlanFixture` the platform's `cmd/api` decodes and passes to
 * `pkg/app.Config.PlanFixtures`. This env var is never set outside a test:
 * production configures `ORCHESTRA_LLM_BASE_URL` instead, which makes the
 * platform ignore it entirely.
 *
 * Ports are picked free at runtime rather than fixed, so this suite cannot
 * collide with a developer's own running processes (8080/8081/8082/5173).
 */

interface RunningService {
  readonly process: ChildProcess;
  readonly port: number;
}

/** A TCP port nothing is listening on right now. */
function freePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const server = createServer();

    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const address = server.address();

      if (address === null || typeof address === "string") {
        reject(new Error("could not read the allocated port"));

        return;
      }

      const { port } = address;

      server.close(() => {
        resolve(port);
      });
    });
  });
}

/** Poll url until it answers with a 2xx status, or give up after timeoutMs. */
async function waitForReady(url: string, timeoutMs: number): Promise<void> {
  const deadline = Date.now() + timeoutMs;

  for (;;) {
    try {
      const response = await fetch(url);

      if (response.ok) return;
    } catch {
      // Not listening yet - keep polling.
    }

    if (Date.now() > deadline) {
      throw new Error(`${url} did not become ready within ${timeoutMs}ms`);
    }

    await delay(200);
  }
}

/** Start a built binary with the given env merged over the current one. */
function startBinary(command: string, env: Record<string, string>): ChildProcess {
  const child = spawn(command, [], {
    cwd: new URL("..", import.meta.url).pathname,
    env: { ...process.env, ...env },
    stdio: ["ignore", "pipe", "pipe"],
  });

  child.on("error", (error) => {
    throw error;
  });

  return child;
}

async function stop(service: RunningService | undefined): Promise<void> {
  if (service === undefined) return;

  service.process.kill();

  await new Promise<void>((resolve) => {
    service.process.once("exit", () => {
      resolve();
    });
  });
}

/**
 * The platform's RunningService, once beforeAll has set it. A helper rather
 * than a conditional inside a test body: vitest's no-conditional-in-test
 * rule keeps a test itself straight-line, so the "did it start" check lives
 * here instead.
 */
function requirePlatform(): RunningService {
  if (platform === undefined) {
    throw new Error("the platform did not start");
  }

  return platform;
}

/** The shape this suite reads out of a POST /api/plan response body. */
interface PlanResponseBody {
  readonly kind: string;
  readonly component: string;
  readonly source: { readonly service: string; readonly operationId: string };
  readonly data: { readonly items: readonly unknown[] };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isPlanResponseBody(value: unknown): value is PlanResponseBody {
  if (!isRecord(value)) return false;
  if (typeof value["kind"] !== "string" || typeof value["component"] !== "string") return false;

  const source = value["source"];
  const data = value["data"];

  if (!isRecord(source) || typeof source["service"] !== "string") return false;
  if (typeof source["operationId"] !== "string") return false;

  return isRecord(data) && Array.isArray(data["items"]);
}

/**
 * Parses a POST /api/plan response body without an unsafe type assertion:
 * `response.json()` returns `unknown` here (no "dom" lib is configured for
 * this Node-only package), so this is the one place that narrows it, by
 * runtime check rather than by `as`.
 */
function parsePlanResponse(value: unknown): PlanResponseBody {
  if (!isPlanResponseBody(value)) {
    throw new Error(`unexpected /api/plan response shape: ${JSON.stringify(value)}`);
  }

  return value;
}

let inventory: RunningService | undefined;
let attendance: RunningService | undefined;
let platform: RunningService | undefined;

beforeAll(async () => {
  const [inventoryPort, attendancePort, platformPort] = await Promise.all([
    freePort(),
    freePort(),
    freePort(),
  ]);

  const inventoryProcess = startBinary("../services/inventory/bin/api", {
    ORCHESTRA_PORT: String(inventoryPort),
  });
  const attendanceProcess = startBinary("../services/attendance/bin/api", {
    ORCHESTRA_PORT: String(attendancePort),
  });

  inventory = { process: inventoryProcess, port: inventoryPort };
  attendance = { process: attendanceProcess, port: attendancePort };

  await Promise.all([
    waitForReady(`http://127.0.0.1:${inventoryPort}/openapi.yaml`, 10_000),
    waitForReady(`http://127.0.0.1:${attendancePort}/openapi.yaml`, 10_000),
  ]);

  const planFixtures = [
    { query: "在庫の一覧を見せて", service: "inventory", operationId: "ListInventoryItems" },
  ];

  // A file in its own temporary directory, per docs/specs/workspaces.md
  // section 6: this suite must not see workspaces another suite wrote, and
  // ORCHESTRA_DB_PATH has no default (internal/infra/config.ErrMissingDBPath)
  // for the platform to fall back to instead.
  const dbPath = join(mkdtempSync(join(tmpdir(), "orchestra-e2e-")), "workspaces.db");

  const platformProcess = startBinary("../services/platform/bin/api", {
    ORCHESTRA_PORT: String(platformPort),
    ORCHESTRA_SERVICES:
      `inventory=http://127.0.0.1:${inventoryPort},` +
      `attendance=http://127.0.0.1:${attendancePort}`,
    ORCHESTRA_PLAN_FIXTURES: JSON.stringify(planFixtures),
    ORCHESTRA_DB_PATH: dbPath,
  });

  platform = { process: platformProcess, port: platformPort };

  await waitForReady(`http://127.0.0.1:${platformPort}/api/health`, 10_000);
}, 30_000);

afterAll(async () => {
  await Promise.all([stop(inventory), stop(attendance), stop(platform)]);
});

describe("the built product answers a question end to end (AC-E-101)", () => {
  it("plans a list question against the running inventory service and renders a table", async () => {
    const port = requirePlatform().port;

    const response = await fetch(`http://127.0.0.1:${port}/api/plan`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ query: "在庫の一覧を見せて" }),
    });

    expect(response.status).toBe(200);

    const body = parsePlanResponse(await response.json());

    expect(body.kind).toBe("result");
    expect(body.component).toBe("table");
    expect(body.source).toEqual({ service: "inventory", operationId: "ListInventoryItems" });
    expect(Array.isArray(body.data.items)).toBe(true);
    expect(body.data.items.length).toBeGreaterThan(0);
  });
});
