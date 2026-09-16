import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterAll, beforeAll, describe, expect, it } from "vite-plus/test";

import { signIn, withSession, type Session } from "./helpers/auth";
import { freePort, startBinary, stop, waitForReady, type RunningService } from "./helpers/process";

/**
 * Process-level end to end proof of the 4xx-becomes-none fix (2026-09-16
 * dev-stack defect: /tmp/orchestra-platform.log 2026-09-16T20:52:15 shows
 * the platform answering /api/plan with 500 after the inventory service
 * answered 404 "no item exists with this id" to a planner-chosen
 * GetInventoryItem(id: itm-999) call - a service saying "not found" is an
 * answer, not a platform failure). itm-999 (not att-002, the fixture's
 * original id) so the id still matches inventory's own contract pattern
 * (`^itm-[0-9]+$`, added for TODO.md's "real-attendance-detail") and
 * reaches this 404, rather than a 400 from the request validation
 * middleware the pattern now enables - a mismatched-service id is what
 * usecase.idAffinity now catches before the pick, not what this suite
 * tests.
 *
 * Split from orchestration.test.ts (which already covers the AC-E-101 call
 * and ask paths against the same two dummy services) only to stay under
 * `harness/quality/eslint`'s max-lines-per-file; the setup below is the
 * same pattern, trimmed to what this one fixture needs. See
 * Orchestrator.resultForInvokeError (internal/usecase/orchestrator.go) for
 * the rule this proves, and messageForServiceError for the message shape.
 */

function requirePlatform(): RunningService {
  if (platform === undefined) {
    throw new Error("the platform did not start");
  }

  return platform;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/** The shape this suite reads out of a POST /api/plan response body when it is a none (kind: "none"). */
interface NoneResponseBody {
  readonly kind: string;
  readonly message: string;
}

function isNoneResponseBody(value: unknown): value is NoneResponseBody {
  if (!isRecord(value)) return false;

  return typeof value["kind"] === "string" && typeof value["message"] === "string";
}

/**
 * Parses a POST /api/plan response body without an unsafe type assertion,
 * the same way orchestration.test.ts's own parsePlanResponse does.
 */
function parseNoneResponse(value: unknown): NoneResponseBody {
  if (!isNoneResponseBody(value)) {
    throw new Error(`unexpected /api/plan none response shape: ${JSON.stringify(value)}`);
  }

  return value;
}

let inventory: RunningService | undefined;
let platform: RunningService | undefined;
let session: Session | undefined;

function requireSession(): Session {
  if (session === undefined) {
    throw new Error("signing in did not happen");
  }

  return session;
}

beforeAll(async () => {
  const [inventoryPort, platformPort] = await Promise.all([freePort(), freePort()]);

  const inventoryProcess = startBinary("../services/inventory/bin/api", {
    ORCHESTRA_PORT: String(inventoryPort),
  });
  inventory = { process: inventoryProcess, port: inventoryPort };

  await waitForReady(`http://127.0.0.1:${inventoryPort}/openapi.yaml`, 10_000);

  // "itm-999" is not seeded by the real inventory service's dummy store
  // (services/inventory), so this reaches Orchestrator.resultForInvokeError
  // through a real HTTP round trip to that service, not a stub - and,
  // unlike the dev-stack defect's own att-002, matches inventory's own
  // id pattern, so the request validation middleware lets it through to a
  // real 404 rather than rejecting it with 400.
  const planFixtures = [
    {
      query: "itm-999の内容",
      service: "inventory",
      operationId: "GetInventoryItem",
      args: { id: "itm-999" },
    },
  ];

  const dbPath = join(mkdtempSync(join(tmpdir(), "orchestra-e2e-")), "workspaces.db");

  const platformProcess = startBinary("../services/platform/bin/api", {
    ORCHESTRA_PORT: String(platformPort),
    ORCHESTRA_SERVICES: `inventory=http://127.0.0.1:${inventoryPort}`,
    ORCHESTRA_PLAN_FIXTURES: JSON.stringify(planFixtures),
    ORCHESTRA_DB_PATH: dbPath,
    ORCHESTRA_ADMIN_PASSWORD: "e2e-admin-password",
    ORCHESTRA_SECURE_COOKIE: "false",
  });

  platform = { process: platformProcess, port: platformPort };

  await waitForReady(`http://127.0.0.1:${platformPort}/api/health`, 10_000);

  session = await signIn(`http://127.0.0.1:${platformPort}`, "admin", "e2e-admin-password");
}, 30_000);

afterAll(async () => {
  await Promise.all([stop(inventory), stop(platform)]);
});

describe("a service's 4xx answer is a result, not a platform failure (dev-stack defect, 2026-09-16)", () => {
  it("reports GetInventoryItem's 404 as a none result carrying the service's own message", async () => {
    const port = requirePlatform().port;

    const response = await fetch(
      `http://127.0.0.1:${port}/api/plan`,
      withSession(requireSession(), {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ query: "itm-999の内容", turns: [], thinking: false }),
      }),
    );

    expect(response.status).toBe(200);

    const body = parseNoneResponse(await response.json());

    expect(body.kind).toBe("none");
    expect(body.message).toBe(
      "在庫管理 の 在庫アイテムの詳細 は「no item exists with this id」と答えました。",
    );
  });
});
