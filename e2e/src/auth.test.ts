import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterAll, beforeAll, describe, expect, it } from "vite-plus/test";

import { signIn, withSession } from "./helpers/auth";
import { freePort, startBinary, stop, waitForReady, type RunningService } from "./helpers/process";
import { asRecord } from "./helpers/wire";

/**
 * Process-level end to end journey for authentication and authorisation
 * (docs/plans/auth.md, Task 6, Step 2; docs/specs/auth.md, AC-A-103,
 * AC-A-104, AC-A-105).
 *
 * Starts both dummy services and the platform from their built binaries
 * (`make build`), the same way `orchestration.test.ts` does. Two non-admin
 * accounts are seeded through `ORCHESTRA_SEED_ACCOUNTS`
 * (`internal/infra/config.Config.SeedAccounts`,
 * `pkg/app.Config.SeedAccounts`'s own doc comment) - there is no HTTP route
 * to create one (docs/specs/auth.md, section 8), and a built binary has no
 * access to `pkg/app.Config` the way a Go acceptance test does.
 *
 * The journey:
 *   1. Sign in as admin, list the accounts, grant one person (yamada) only
 *      the inventory operation.
 *   2. Sign in as yamada: a question the fixture maps to inventory is
 *      answered from it; the same question mapped to attendance is not
 *      (AC-A-103) - and calling the attendance operation directly through
 *      `/api/invoke` is refused the same way an unknown operation would be
 *      (AC-A-104).
 *   3. yamada makes a workspace; the other seeded person (suzuki) neither
 *      lists nor reads it (AC-A-105).
 */

function isAccountOnWire(value: unknown): value is { readonly id: string; readonly name: string } {
  const record = asRecord(value);

  return typeof record["id"] === "string" && typeof record["name"] === "string";
}

/** The id of the seeded account named name, from a GET /api/users response body. */
function findAccountID(value: unknown, name: string): string {
  if (!Array.isArray(value)) {
    throw new TypeError(`unexpected /api/users response shape: ${JSON.stringify(value)}`);
  }

  for (const entry of value) {
    if (isAccountOnWire(entry) && entry.name === name) {
      return entry.id;
    }
  }

  throw new Error(`no account named ${name} among ${JSON.stringify(value)}`);
}

const yamadaName = "yamada";
const yamadaPassword = "yamada-e2e-password";
const suzukiName = "suzuki";
const suzukiPassword = "suzuki-e2e-password";

let inventory: RunningService | undefined;
let attendance: RunningService | undefined;
let platform: RunningService | undefined;

/** The platform's RunningService, once beforeAll has set it. See orchestration.test.ts's own. */
function requirePlatform(): RunningService {
  if (platform === undefined) {
    throw new Error("the platform did not start");
  }

  return platform;
}

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

  const planFixtures = [
    { query: "在庫の一覧を見せて", service: "inventory", operationId: "ListInventoryItems" },
    { query: "出勤記録を見せて", service: "attendance", operationId: "ListAttendanceRecords" },
  ];

  const seedAccounts = [
    { name: yamadaName, password: yamadaPassword, role: "user" },
    { name: suzukiName, password: suzukiPassword, role: "user" },
  ];

  // A file in its own temporary directory, per docs/specs/workspaces.md
  // section 6: this suite must not see accounts or workspaces another
  // suite wrote.
  const dbPath = join(mkdtempSync(join(tmpdir(), "orchestra-e2e-auth-")), "workspaces.db");

  platform = {
    process: startBinary("../services/platform/bin/api", {
      ORCHESTRA_PORT: String(platformPort),
      ORCHESTRA_SERVICES:
        `inventory=http://127.0.0.1:${inventoryPort},` +
        `attendance=http://127.0.0.1:${attendancePort}`,
      ORCHESTRA_PLAN_FIXTURES: JSON.stringify(planFixtures),
      ORCHESTRA_SEED_ACCOUNTS: JSON.stringify(seedAccounts),
      ORCHESTRA_DB_PATH: dbPath,
      ORCHESTRA_ADMIN_PASSWORD: "e2e-auth-admin-password",
      // Plain HTTP (127.0.0.1, no TLS): see orchestration.test.ts's own
      // comment on this variable.
      ORCHESTRA_SECURE_COOKIE: "false",
    }),
    port: platformPort,
  };

  await waitForReady(`http://127.0.0.1:${platformPort}/api/health`, 10_000);
}, 30_000);

afterAll(async () => {
  await Promise.all([stop(inventory), stop(attendance), stop(platform)]);
});

describe("a person reaches only the service they were granted (AC-A-103, AC-A-104)", () => {
  it("answers from the granted service, refuses the other, and cannot invoke it directly", async () => {
    const baseUrl = `http://127.0.0.1:${requirePlatform().port}`;

    const admin = await signIn(baseUrl, "admin", "e2e-auth-admin-password");

    const accountsResponse = await fetch(`${baseUrl}/api/users`, withSession(admin));

    expect(accountsResponse.status).toBe(200);

    const yamadaID = findAccountID(await accountsResponse.json(), yamadaName);

    // Grant yamada only the inventory operation - not the attendance one
    // the same fixture table also knows about.
    const grantResponse = await fetch(
      `${baseUrl}/api/users/${yamadaID}/permissions`,
      withSession(admin, {
        method: "PUT",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          permissions: [{ service: "inventory", operationId: "ListInventoryItems" }],
        }),
      }),
    );

    expect(grantResponse.status).toBe(204);

    const yamada = await signIn(baseUrl, yamadaName, yamadaPassword);

    const grantedResponse = await fetch(
      `${baseUrl}/api/plan`,
      withSession(yamada, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ query: "在庫の一覧を見せて" }),
      }),
    );

    expect(grantedResponse.status).toBe(200);

    const grantedPlan = asRecord(await grantedResponse.json());

    expect(grantedPlan["kind"]).toBe("result");
    expect(grantedPlan["source"]).toEqual({
      service: "inventory",
      serviceDisplayName: "在庫管理",
      operationId: "ListInventoryItems",
    });

    // The same person asking the fixture's attendance question is not
    // answered from it - the tools the planner was offered name no
    // attendance operation for yamada at all (AC-A-103). Whatever status
    // this comes back as, it never actually answers: no "result" kind, and
    // no data an attendance record could have leaked through.
    const refusedResponse = await fetch(
      `${baseUrl}/api/plan`,
      withSession(yamada, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ query: "出勤記録を見せて" }),
      }),
    );

    expect(refusedResponse.status).not.toBe(200);

    const refusedPlan = asRecord(await refusedResponse.json());

    expect(refusedPlan["kind"]).not.toBe("result");
    expect(refusedPlan["data"]).toBeUndefined();

    // Naming the attendance operation directly through /api/invoke gets
    // the same refusal an operation that does not exist at all would
    // (AC-A-104).
    const invokeResponse = await fetch(
      `${baseUrl}/api/invoke`,
      withSession(yamada, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          service: "attendance",
          operationId: "ListAttendanceRecords",
          args: {},
        }),
      }),
    );

    expect(invokeResponse.status).toBe(400);
  }, 30_000);
});

describe("a workspace one person makes is invisible to another (AC-A-105)", () => {
  it("is not listed or readable by a different signed-in person", async () => {
    const baseUrl = `http://127.0.0.1:${requirePlatform().port}`;

    const yamada = await signIn(baseUrl, yamadaName, yamadaPassword);

    const createResponse = await fetch(
      `${baseUrl}/api/workspaces`,
      withSession(yamada, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ name: "やまだの在庫ボード" }),
      }),
    );

    expect(createResponse.status).toBe(201);

    const created = asRecord(await createResponse.json());
    const createdID = created["id"];

    expect(typeof createdID).toBe("string");

    const suzuki = await signIn(baseUrl, suzukiName, suzukiPassword);

    const listResponse = await fetch(`${baseUrl}/api/workspaces`, withSession(suzuki));

    expect(listResponse.status).toBe(200);

    const list: unknown = await listResponse.json();

    expect(JSON.stringify(list)).not.toContain(createdID);

    const readResponse = await fetch(
      `${baseUrl}/api/workspaces/${String(createdID)}`,
      withSession(suzuki),
    );

    expect(readResponse.status).toBe(404);
  });
});
