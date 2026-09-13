import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterAll, beforeAll, describe, expect, it } from "vite-plus/test";

import { signIn, withSession, type Session } from "./helpers/auth";
import { freePort, startBinary, stop, waitForReady, type RunningService } from "./helpers/process";

/**
 * AC-P-109's own process-level test: a `PATCH` is refused exactly the way
 * `POST /api/workspaces/{id}/panels` is, for both of its own reasons - the
 * panel's (fixed, P13) operation is no longer one the caller may call, or
 * the panel belongs to somebody else - the same 400/404 split
 * `dashboard-permissions.test.ts`'s own AC-P-107 test already exercises
 * for `addPanel`. Its own file, not a describe block added to that one,
 * because that file is already at its own `max-lines` budget.
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

interface WithID {
  readonly id: string;
}

function hasID(value: unknown): value is WithID {
  return isRecord(value) && typeof value["id"] === "string";
}

/** Reads the `id` off a workspace-created or panel-saved response body - both are objects with at least that much. */
function parseWithID(value: unknown): WithID {
  if (!hasID(value)) {
    throw new Error(`unexpected response shape, wanted an id: ${JSON.stringify(value)}`);
  }

  return value;
}

const narrowUserName = "narrow-user";
const narrowUserPassword = "narrow-user-e2e-password";

let inventory: RunningService | undefined;
let platform: RunningService | undefined;
let admin: Session;

beforeAll(async () => {
  const [inventoryPort, platformPort] = await Promise.all([freePort(), freePort()]);

  inventory = {
    process: startBinary("../services/inventory/bin/api", {
      ORCHESTRA_PORT: String(inventoryPort),
    }),
    port: inventoryPort,
  };

  await waitForReady(`http://127.0.0.1:${inventoryPort}/openapi.yaml`, 10_000);

  const dbPath = join(
    mkdtempSync(join(tmpdir(), "orchestra-e2e-dashboard-update-perms-")),
    "dashboard.db",
  );

  platform = {
    process: startBinary("../services/platform/bin/api", {
      ORCHESTRA_PORT: String(platformPort),
      ORCHESTRA_SERVICES: `inventory=http://127.0.0.1:${inventoryPort}`,
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
  await Promise.all([stop(inventory), stop(platform)]);
});

/** The platform's own base URL, once beforeAll has started it. */
function baseUrl(): string {
  if (platform === undefined) {
    throw new Error("the platform did not start");
  }

  return `http://127.0.0.1:${String(platform.port)}`;
}

async function grantInventoryListing(userID: string): Promise<void> {
  await fetch(
    `${baseUrl()}/api/users/${userID}/permissions`,
    withSession(admin, {
      method: "PUT",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        permissions: [{ service: "inventory", operationId: "ListInventoryItems" }],
      }),
    }),
  );
}

async function createWorkspace(session: Session, name: string): Promise<WithID> {
  const response = await fetch(
    `${baseUrl()}/api/workspaces`,
    withSession(session, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ name }),
    }),
  );

  return parseWithID(await response.json());
}

async function addInventoryPanel(session: Session, workspaceID: string): Promise<WithID> {
  const response = await fetch(
    `${baseUrl()}/api/workspaces/${workspaceID}/panels`,
    withSession(session, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        service: "inventory",
        operationId: "ListInventoryItems",
        args: {},
        component: "table",
        title: "在庫一覧",
      }),
    }),
  );

  return parseWithID(await response.json());
}

describe("a PATCH is refused exactly the way adding a panel is (AC-P-109)", () => {
  it("refuses one naming an operation the panel's owner may no longer call", async () => {
    const accountsResponse = await fetch(`${baseUrl()}/api/users`, withSession(admin));
    const narrowUserID = findAccountID(await accountsResponse.json(), narrowUserName);

    await grantInventoryListing(narrowUserID);

    const narrowUser = await signIn(baseUrl(), narrowUserName, narrowUserPassword);
    const workspace = await createWorkspace(narrowUser, "権限が変わるワークスペース");
    const panel = await addInventoryPanel(narrowUser, workspace.id);

    // Revoke the operation this panel already calls - permissions can
    // change after a panel is made, and updatePanel re-checks them rather
    // than trusting the check addPanel already made.
    await fetch(
      `${baseUrl()}/api/users/${narrowUserID}/permissions`,
      withSession(admin, {
        method: "PUT",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ permissions: [] }),
      }),
    );

    const response = await fetch(
      `${baseUrl()}/api/workspaces/${workspace.id}/panels/${panel.id}`,
      withSession(narrowUser, {
        method: "PATCH",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ title: "新しいタイトル" }),
      }),
    );

    expect(response.status).toBe(400);
  });

  it("refuses one naming a panel belonging to somebody else - not found, not forbidden", async () => {
    const workspace = await createWorkspace(admin, "管理者のワークスペース");
    const panel = await addInventoryPanel(admin, workspace.id);

    const accountsResponse = await fetch(`${baseUrl()}/api/users`, withSession(admin));
    const narrowUserID = findAccountID(await accountsResponse.json(), narrowUserName);

    await grantInventoryListing(narrowUserID);

    const stranger = await signIn(baseUrl(), narrowUserName, narrowUserPassword);

    const response = await fetch(
      `${baseUrl()}/api/workspaces/${workspace.id}/panels/${panel.id}`,
      withSession(stranger, {
        method: "PATCH",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ title: "乗っ取り" }),
      }),
    );

    expect(response.status).toBe(404);
  });
});
