import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterAll, beforeAll, describe, expect, it } from "vite-plus/test";

import { signIn, withSession, type Session } from "./helpers/auth";
import { freePort, startBinary, stop, waitForReady, type RunningService } from "./helpers/process";

/**
 * AC-P-107's own process-level test - a panel over an operation the
 * calling person may not call is refused the same way `/api/invoke`
 * refuses it - split out of `dashboard.test.ts`, which is already at its
 * `max-lines` budget.
 *
 * Both dummy services are configured (unlike `dashboard.test.ts`) so there
 * is a real second service to be refused, and a non-admin account is
 * seeded through `ORCHESTRA_SEED_ACCOUNTS` - the same way `auth.test.ts`
 * does, since there is no HTTP route to create one
 * (docs/specs/auth.md, section 8) - and granted only the inventory
 * operation, never the attendance one.
 */

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isAccount(value: unknown): value is { readonly id: string; readonly name: string } {
  return isRecord(value) && typeof value["id"] === "string" && typeof value["name"] === "string";
}

/** The id of the seeded account named name, from a GET /api/users response body. */
function findAccountID(value: unknown, name: string): string {
  if (!Array.isArray(value) || !value.every((entry) => isAccount(entry))) {
    throw new Error(`unexpected /api/users response shape: ${JSON.stringify(value)}`);
  }

  const account = value.find((entry) => entry.name === name);

  if (account === undefined) {
    throw new Error(`no account named ${name} among ${JSON.stringify(value)}`);
  }

  return account.id;
}

interface WorkspaceCreatedOnWire {
  readonly id: string;
}

function isWorkspaceCreated(value: unknown): value is WorkspaceCreatedOnWire {
  return isRecord(value) && typeof value["id"] === "string";
}

function parseWorkspaceCreated(value: unknown): WorkspaceCreatedOnWire {
  if (!isWorkspaceCreated(value)) {
    throw new Error(`unexpected workspace-created response shape: ${JSON.stringify(value)}`);
  }

  return value;
}

const narrowUserName = "narrow-user";
const narrowUserPassword = "narrow-user-e2e-password";

let inventory: RunningService | undefined;
let attendance: RunningService | undefined;
let platform: RunningService | undefined;
let admin: Session;

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

  const dbPath = join(
    mkdtempSync(join(tmpdir(), "orchestra-e2e-dashboard-permissions-")),
    "dashboard.db",
  );

  platform = {
    process: startBinary("../services/platform/bin/api", {
      ORCHESTRA_PORT: String(platformPort),
      ORCHESTRA_SERVICES:
        `inventory=http://127.0.0.1:${inventoryPort},` +
        `attendance=http://127.0.0.1:${attendancePort}`,
      ORCHESTRA_SEED_ACCOUNTS: JSON.stringify([
        { name: narrowUserName, password: narrowUserPassword, role: "user" },
      ]),
      ORCHESTRA_DB_PATH: dbPath,
      ORCHESTRA_ADMIN_PASSWORD: "e2e-admin-password",
      // Plain HTTP (127.0.0.1, no TLS): see orchestration.test.ts's own
      // comment on this variable.
      ORCHESTRA_SECURE_COOKIE: "false",
    }),
    port: platformPort,
  };

  await waitForReady(`http://127.0.0.1:${platformPort}/api/health`, 10_000);

  admin = await signIn(`http://127.0.0.1:${platformPort}`, "admin", "e2e-admin-password");
}, 30_000);

afterAll(async () => {
  await Promise.all([stop(inventory), stop(attendance), stop(platform)]);
});

/** The platform's own base URL, once beforeAll has started it. */
function baseUrl(): string {
  if (platform === undefined) {
    throw new Error("the platform did not start");
  }

  return `http://127.0.0.1:${String(platform.port)}`;
}

describe("a person may not build a panel over an operation they may not call (AC-P-107)", () => {
  it("refuses it, the same way /api/invoke would", async () => {
    const accountsResponse = await fetch(`${baseUrl()}/api/users`, withSession(admin));
    const narrowUserID = findAccountID(await accountsResponse.json(), narrowUserName);

    // Grant only the inventory operation - never the attendance one this
    // platform also has configured, so the refusal below is a real
    // permission boundary and not just an absent service.
    await fetch(
      `${baseUrl()}/api/users/${narrowUserID}/permissions`,
      withSession(admin, {
        method: "PUT",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          permissions: [{ service: "inventory", operationId: "ListInventoryItems" }],
        }),
      }),
    );

    const narrowUser = await signIn(baseUrl(), narrowUserName, narrowUserPassword);

    const createWorkspace = await fetch(
      `${baseUrl()}/api/workspaces`,
      withSession(narrowUser, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ name: "権限外のワークスペース" }),
      }),
    );
    const workspace = parseWorkspaceCreated(await createWorkspace.json());

    const response = await fetch(
      `${baseUrl()}/api/workspaces/${workspace.id}/panels`,
      withSession(narrowUser, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          service: "attendance",
          operationId: "ListAttendanceRecords",
          args: {},
          component: "table",
          title: "権限のない操作",
        }),
      }),
    );

    expect(response.status).toBe(400);
  });
});
