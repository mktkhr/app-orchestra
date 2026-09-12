import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterAll, beforeAll, describe, expect, it } from "vite-plus/test";

import { signIn, withSession, type Session } from "./helpers/auth";
import { freePort, startBinary, stop, waitForReady, type RunningService } from "./helpers/process";
import { asRecord } from "./helpers/wire";

/**
 * Process-level end to end journey for multi-turn context
 * (docs/plans/context.md, Task 4, Step 1; docs/specs/context.md, AC-M-101).
 *
 * Started the same way `orchestration.test.ts` is - built binaries, real
 * TCP, the stub planner over `ORCHESTRA_PLAN_FIXTURES` - so this suite
 * proves what actually ships (the platform threading `turns` from the wire
 * through to the planner: `pkg/app`, `internal/adapter/handler/plan.go`,
 * `internal/usecase/orchestrator.go`) rather than the stub's own unit
 * tests, which prove `internal/adapter/planner/stub` in isolation.
 *
 * The stub cannot infer which service a service-less follow-up means - it
 * performs no reasoning over anything, by design (the global constraint
 * that `make check` never calls a real LLM). What this suite fixes instead
 * is `stub.Key` carrying `Turns` (docs/plans/context.md, Task 4): two
 * fixture rows share the same follow-up `Query` and differ only in the
 * conversation's turns, so the platform being handed the same turns a real
 * planner would render into its prompt is what selects between them. That
 * the stub was extended to read turns at all - rather than only ever
 * ignoring them, as `docs/plans/context.md` Task 1 originally left it - is
 * this task's own decision; see stub.go's package doc and DECISIONS.md for
 * why a real model's actual ability to do this is measured separately, by
 * hand, not asserted by this test.
 */

interface PlanResponseBody {
  readonly kind: string;
  readonly source?: { readonly service: string; readonly operationId: string };
}

function isPlanResponseBody(value: unknown): value is PlanResponseBody {
  const record = asRecord(value);

  if (typeof record["kind"] !== "string") return false;

  const source = record["source"];

  if (source === undefined) return true;

  const sourceRecord = asRecord(source);

  return (
    typeof sourceRecord["service"] === "string" && typeof sourceRecord["operationId"] === "string"
  );
}

function parsePlanResponse(value: unknown): PlanResponseBody {
  if (!isPlanResponseBody(value)) {
    throw new Error(`unexpected /api/plan response shape: ${JSON.stringify(value)}`);
  }

  return value;
}

let inventory: RunningService | undefined;
let attendance: RunningService | undefined;
let platform: RunningService | undefined;
let session: Session | undefined;

function requirePlatform(): RunningService {
  if (platform === undefined) {
    throw new Error("the platform did not start");
  }

  return platform;
}

function requireSession(): Session {
  if (session === undefined) {
    throw new Error("signing in did not happen");
  }

  return session;
}

const inventoryQuestion = "在庫の一覧を見せて";
const attendanceQuestion = "出勤記録を見せて";
const followUpQuestion = "検品保留のものだけ見せて";

beforeAll(async () => {
  const [inventoryPort, attendancePort, platformPort] = await Promise.all([
    freePort(),
    freePort(),
    freePort(),
  ]);

  inventory = {
    process: startBinary("../services/inventory/bin/api", {
      ORCHESTRA_PORT: String(inventoryPort),
    }),
    port: inventoryPort,
  };
  attendance = {
    process: startBinary("../services/attendance/bin/api", {
      ORCHESTRA_PORT: String(attendancePort),
    }),
    port: attendancePort,
  };

  await Promise.all([
    waitForReady(`http://127.0.0.1:${inventoryPort}/openapi.yaml`, 10_000),
    waitForReady(`http://127.0.0.1:${attendancePort}/openapi.yaml`, 10_000),
  ]);

  // Two fixtures share followUpQuestion's exact text and differ only in
  // Turns: the same question, asked after a different conversation,
  // resolves to a different service. That is the mechanism AC-M-101 asks
  // the platform to have; a stub cannot demonstrate it any other way,
  // since it never reads a question's meaning at all.
  const planFixtures = [
    { query: inventoryQuestion, service: "inventory", operationId: "ListInventoryItems" },
    { query: attendanceQuestion, service: "attendance", operationId: "ListAttendanceRecords" },
    {
      query: followUpQuestion,
      turns: [{ service: "inventory", operationId: "ListInventoryItems" }],
      service: "inventory",
      operationId: "ListInventoryItems",
    },
    {
      query: followUpQuestion,
      turns: [{ service: "attendance", operationId: "ListAttendanceRecords" }],
      service: "attendance",
      operationId: "ListAttendanceRecords",
    },
  ];

  const dbPath = join(mkdtempSync(join(tmpdir(), "orchestra-e2e-context-")), "workspaces.db");

  platform = {
    process: startBinary("../services/platform/bin/api", {
      ORCHESTRA_PORT: String(platformPort),
      ORCHESTRA_SERVICES:
        `inventory=http://127.0.0.1:${inventoryPort},` +
        `attendance=http://127.0.0.1:${attendancePort}`,
      ORCHESTRA_PLAN_FIXTURES: JSON.stringify(planFixtures),
      ORCHESTRA_DB_PATH: dbPath,
      ORCHESTRA_ADMIN_PASSWORD: "e2e-context-admin-password",
      // Plain HTTP (127.0.0.1, no TLS): see orchestration.test.ts's own
      // comment on this variable.
      ORCHESTRA_SECURE_COOKIE: "false",
    }),
    port: platformPort,
  };

  await waitForReady(`http://127.0.0.1:${platformPort}/api/health`, 10_000);

  session = await signIn(`http://127.0.0.1:${platformPort}`, "admin", "e2e-context-admin-password");
}, 30_000);

afterAll(async () => {
  await Promise.all([stop(inventory), stop(attendance), stop(platform)]);
});

/** Posts query with turns (default none) against the running platform. */
async function postPlan(query: string, turns: readonly unknown[] = []): Promise<PlanResponseBody> {
  const response = await fetch(
    `http://127.0.0.1:${requirePlatform().port}/api/plan`,
    withSession(requireSession(), {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ query, turns }),
    }),
  );

  expect(response.status).toBe(200);

  return parsePlanResponse(await response.json());
}

describe("a follow-up naming no service is answered from the previous one (AC-M-101)", () => {
  it("asks about inventory, then a follow-up naming no service, and stays on inventory", async () => {
    const first = await postPlan(inventoryQuestion);

    expect(first.kind).toBe("result");
    expect(first.source).toEqual({ service: "inventory", operationId: "ListInventoryItems" });

    const turns = [
      {
        question: inventoryQuestion,
        kind: "result",
        service: "inventory",
        operationId: "ListInventoryItems",
      },
    ];

    const followUp = await postPlan(followUpQuestion, turns);

    expect(followUp.kind).toBe("result");
    expect(followUp.source).toEqual({ service: "inventory", operationId: "ListInventoryItems" });
  });

  it("asks about attendance, then the very same follow-up wording, and stays on attendance", async () => {
    const first = await postPlan(attendanceQuestion);

    expect(first.kind).toBe("result");
    expect(first.source).toEqual({ service: "attendance", operationId: "ListAttendanceRecords" });

    const turns = [
      {
        question: attendanceQuestion,
        kind: "result",
        service: "attendance",
        operationId: "ListAttendanceRecords",
      },
    ];

    const followUp = await postPlan(followUpQuestion, turns);

    expect(followUp.kind).toBe("result");
    expect(followUp.source).toEqual({
      service: "attendance",
      operationId: "ListAttendanceRecords",
    });
  });

  it("answers the same wording with nothing when no conversation came before it", async () => {
    const followUp = await postPlan(followUpQuestion);

    expect(followUp.kind).toBe("none");
  });
});
