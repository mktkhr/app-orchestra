import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterAll, describe, expect, it } from "vite-plus/test";

import { signIn, withSession, type Session } from "./helpers/auth";
import { freePort, startBinary, stop, waitForReady, type RunningService } from "./helpers/process";

/**
 * Process-level end to end suite for workspaces (AC-W-105,
 * docs/plans/workspaces.md Task 7 Step 1-2).
 *
 * Creates a workspace and a panel over real HTTP against the built
 * platform binary, stops that process, starts a fresh one on the same
 * `ORCHESTRA_DB_PATH` file, and reads the workspace back - proving it
 * survives a restart of the platform, not just a page reload.
 *
 * Free port allocation and starting/stopping a built binary come from
 * `helpers/process.ts`, shared with `orchestration.test.ts` and
 * `auth.test.ts` - each file still starts and stops its own processes on
 * its own schedule (this one restarts the platform mid-test, which the
 * others never do), only the plumbing underneath is common now.
 *
 * The database is a file inside its own `mkdtempSync` directory, per
 * `docs/specs/workspaces.md` section 6 / W3: this suite's workspaces are
 * invisible to `orchestration.test.ts`'s suite (which never touches
 * `/api/workspaces` at all) and to a developer's own running platform,
 * because each gets its own temporary directory and `ORCHESTRA_DB_PATH`
 * has no default (`internal/infra/config.ErrMissingDBPath`) to fall back
 * to instead.
 */

/**
 * Starts the platform binary on a free port, over the given env, waits for
 * it to answer /api/health, and signs in as admin (docs/plans/auth.md
 * Task 6, Step 1) - every route but GET /api/health and POST /api/session
 * now answers 401 without a session (AC-A-102).
 */
async function startPlatform(
  env: Record<string, string>,
): Promise<{ readonly service: RunningService; readonly session: Session }> {
  const port = await freePort();
  const platformProcess = startBinary("../services/platform/bin/api", {
    ...env,
    ORCHESTRA_PORT: String(port),
  });

  await waitForReady(`http://127.0.0.1:${port}/api/health`, 10_000);

  const session = await signIn(`http://127.0.0.1:${port}`, "admin", "e2e-admin-password");

  return { service: { process: platformProcess, port }, session };
}

interface WorkspaceOnWire {
  readonly id: string;
  readonly name: string;
  readonly panels: readonly {
    readonly id: string;
    readonly service: string;
    readonly operationId: string;
    readonly title: string;
  }[];
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isWorkspaceOnWire(value: unknown): value is WorkspaceOnWire {
  return (
    isRecord(value) &&
    typeof value["id"] === "string" &&
    typeof value["name"] === "string" &&
    Array.isArray(value["panels"])
  );
}

function parseWorkspace(value: unknown): WorkspaceOnWire {
  if (!isWorkspaceOnWire(value)) {
    throw new Error(`unexpected workspace response shape: ${JSON.stringify(value)}`);
  }

  return value;
}

interface WorkspaceCreatedOnWire {
  readonly id: string;
  readonly name: string;
}

function isWorkspaceCreatedOnWire(value: unknown): value is WorkspaceCreatedOnWire {
  return isRecord(value) && typeof value["id"] === "string" && typeof value["name"] === "string";
}

function parseWorkspaceCreated(value: unknown): WorkspaceCreatedOnWire {
  if (!isWorkspaceCreatedOnWire(value)) {
    throw new Error(`unexpected workspace-created response shape: ${JSON.stringify(value)}`);
  }

  return value;
}

let inventory: RunningService | undefined;
const platforms: RunningService[] = [];

afterAll(async () => {
  await Promise.all([stop(inventory), ...platforms.map((service) => stop(service))]);
});

describe("a workspace survives a restart of the platform (AC-W-105)", () => {
  it("reads back a workspace and its panel after the platform is stopped and started again", async () => {
    const inventoryPort = await freePort();

    inventory = {
      process: startBinary("../services/inventory/bin/api", {
        ORCHESTRA_PORT: String(inventoryPort),
      }),
      port: inventoryPort,
    };

    await waitForReady(`http://127.0.0.1:${inventoryPort}/openapi.yaml`, 10_000);

    // A file in its own temporary directory: this suite's workspaces
    // must not be visible to any other suite, and must not depend on
    // the platform staying up between the two starts below.
    const dbPath = join(mkdtempSync(join(tmpdir(), "orchestra-e2e-workspaces-")), "workspaces.db");
    const sharedEnv = {
      ORCHESTRA_SERVICES: `inventory=http://127.0.0.1:${inventoryPort}`,
      ORCHESTRA_DB_PATH: dbPath,
      // ORCHESTRA_ADMIN_PASSWORD has no default either
      // (internal/infra/config.ErrMissingAdminPassword) - a fixed value is
      // fine here, since this suite signs in as this same admin.
      ORCHESTRA_ADMIN_PASSWORD: "e2e-admin-password",
      // Plain HTTP (127.0.0.1, no TLS): see orchestration.test.ts's own
      // comment on this variable.
      ORCHESTRA_SECURE_COOKIE: "false",
    };

    // Not pushed to `platforms`: the test stops it itself below, and
    // `stop`'s "exit" listener only fires for a process that has not
    // exited yet - attaching a second one in afterAll after that would
    // wait forever.
    const first = await startPlatform(sharedEnv);

    const createResponse = await fetch(
      `http://127.0.0.1:${first.service.port}/api/workspaces`,
      withSession(first.session, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ name: "在庫ボード" }),
      }),
    );

    expect(createResponse.status).toBe(201);

    const created = parseWorkspaceCreated(await createResponse.json());

    const panelResponse = await fetch(
      `http://127.0.0.1:${first.service.port}/api/workspaces/${created.id}/panels`,
      withSession(first.session, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          service: "inventory",
          operationId: "ListInventoryItems",
          args: {},
          component: "table",
          title: "検品保留の在庫",
        }),
      }),
    );

    expect(panelResponse.status).toBe(201);

    // Stop the platform - the same process a restart or a deploy would
    // do - and start a fresh one on the same database file.
    await stop(first.service);

    const second = await startPlatform(sharedEnv);

    platforms.push(second.service);

    const readBack = await fetch(
      `http://127.0.0.1:${second.service.port}/api/workspaces/${created.id}`,
      withSession(second.session),
    );

    expect(readBack.status).toBe(200);

    const workspace = parseWorkspace(await readBack.json());

    expect(workspace.id).toBe(created.id);
    expect(workspace.name).toBe("在庫ボード");
    expect(workspace.panels).toHaveLength(1);
    expect(workspace.panels[0]).toMatchObject({
      service: "inventory",
      operationId: "ListInventoryItems",
      title: "検品保留の在庫",
    });
  }, 30_000);
});
