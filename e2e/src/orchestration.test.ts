import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterAll, beforeAll, describe, expect, it } from "vite-plus/test";

import { signIn, withSession, type Session } from "./helpers/auth";
import { freePort, startBinary, stop, waitForReady, type RunningService } from "./helpers/process";

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

/**
 * The shape this suite reads out of a POST /api/plan response body when it
 * is an ask (kind: "ask") - the ask_user degradation fix's own two outcomes
 * (askDegrade, internal/usecase/orchestrator_ask.go, 2026-09-16): rule 2's
 * options carried verbatim, or rule 3's plain question with neither param
 * nor options at all.
 */
interface AskResponseBody {
  readonly kind: string;
  readonly question: string;
  readonly param?: string;
  readonly options?: ReadonlyArray<{ readonly value: string; readonly label: string }>;
}

function isAskResponseBody(value: unknown): value is AskResponseBody {
  if (!isRecord(value)) return false;

  return typeof value["kind"] === "string" && typeof value["question"] === "string";
}

/** Parses a POST /api/plan response body as an ask. See parsePlanResponse. */
function parseAskResponse(value: unknown): AskResponseBody {
  if (!isAskResponseBody(value)) {
    throw new Error(`unexpected /api/plan ask response shape: ${JSON.stringify(value)}`);
  }

  return value;
}

let inventory: RunningService | undefined;
let attendance: RunningService | undefined;
let platform: RunningService | undefined;
let session: Session | undefined;

/** The admin Session, once beforeAll has signed in. See requirePlatform. */
function requireSession(): Session {
  if (session === undefined) {
    throw new Error("signing in did not happen");
  }

  return session;
}

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
    // Rule 2 (askDegrade, internal/usecase/orchestrator_ask.go): "kind" is
    // not a parameter ListInventoryItems declares at all, so it is neither
    // an enum (optionsForParam) nor required - and two or more
    // model-supplied options are trusted verbatim as a real ask.
    {
      query: "注文を見たい",
      ask: true,
      question: "受注ですか、発注ですか？",
      param: "kind",
      options: [
        { value: "sales", label: "受注" },
        { value: "purchase", label: "発注" },
      ],
      service: "inventory",
      operationId: "ListInventoryItems",
    },
    // Rule 3: the same non-enum, non-required situation but with no
    // options at all - degrades to a plain question, carrying neither
    // param nor options, rather than the empty form this used to render
    // before the 2026-09-16 fix (TODO.md item 3).
    {
      query: "いつの分の在庫ですか",
      ask: true,
      question: "いつの分ですか？",
      param: "period",
      service: "inventory",
      operationId: "ListInventoryItems",
    },
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
    // ORCHESTRA_ADMIN_PASSWORD has no default either
    // (internal/infra/config.ErrMissingAdminPassword).
    ORCHESTRA_ADMIN_PASSWORD: "e2e-admin-password",
    // This suite runs over plain HTTP (127.0.0.1, no TLS): a Secure
    // cookie would never be stored, and signing in below would appear to
    // work while the session never actually carried (docs/plans/auth.md
    // Task 6, Step 1; internal/infra/config.Config.SecureCookie).
    ORCHESTRA_SECURE_COOKIE: "false",
  });

  platform = { process: platformProcess, port: platformPort };

  await waitForReady(`http://127.0.0.1:${platformPort}/api/health`, 10_000);

  session = await signIn(`http://127.0.0.1:${platformPort}`, "admin", "e2e-admin-password");
}, 30_000);

afterAll(async () => {
  await Promise.all([stop(inventory), stop(attendance), stop(platform)]);
});

describe("the built product answers a question end to end (AC-E-101)", () => {
  it("plans a list question against the running inventory service and renders a table", async () => {
    const port = requirePlatform().port;

    const response = await fetch(
      `http://127.0.0.1:${port}/api/plan`,
      withSession(requireSession(), {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ query: "在庫の一覧を見せて" }),
      }),
    );

    expect(response.status).toBe(200);

    const body = parsePlanResponse(await response.json());

    expect(body.kind).toBe("result");
    expect(body.component).toBe("table");
    expect(body.source).toEqual({
      service: "inventory",
      serviceDisplayName: "在庫管理",
      operationId: "ListInventoryItems",
    });
    expect(Array.isArray(body.data.items)).toBe(true);
    expect(body.data.items.length).toBeGreaterThan(0);
  });

  // The ask_user degradation fix (2026-09-16, TODO.md item 3): a safe
  // operation's ask over a param with no catalogue enum no longer degrades
  // unconditionally to an empty form. These two cases are askDegrade's
  // rule 2 and rule 3 (internal/usecase/orchestrator_ask.go) driven through
  // the built binary, the same way the result case above proves a call.
  it("asks using the model's own options when it supplies two or more (rule 2)", async () => {
    const port = requirePlatform().port;

    const response = await fetch(
      `http://127.0.0.1:${port}/api/plan`,
      withSession(requireSession(), {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ query: "注文を見たい" }),
      }),
    );

    expect(response.status).toBe(200);

    const body = parseAskResponse(await response.json());

    expect(body.kind).toBe("ask");
    expect(body.question).toBe("受注ですか、発注ですか？");
    expect(body.param).toBe("kind");
    expect(body.options).toEqual([
      { value: "sales", label: "受注" },
      { value: "purchase", label: "発注" },
    ]);
  });

  it("degrades to a plain question with no param or options when there is nothing to offer (rule 3)", async () => {
    const port = requirePlatform().port;

    const response = await fetch(
      `http://127.0.0.1:${port}/api/plan`,
      withSession(requireSession(), {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ query: "いつの分の在庫ですか" }),
      }),
    );

    expect(response.status).toBe(200);

    const body = parseAskResponse(await response.json());

    expect(body.kind).toBe("ask");
    expect(body.question).toBe("いつの分ですか？");
    expect(body.param).toBeUndefined();
    expect(body.options).toBeUndefined();
  });
});
