import { type ChildProcess, spawn } from "node:child_process";
import { mkdtempSync } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { setTimeout as delay } from "node:timers/promises";

import { afterAll, describe, expect, it } from "vite-plus/test";

/**
 * Process-level end to end suite for workspaces (AC-W-105,
 * docs/plans/workspaces.md Task 7 Step 1-2).
 *
 * Creates a workspace and a panel over real HTTP against the built
 * platform binary, stops that process, starts a fresh one on the same
 * `ORCHESTRA_DB_PATH` file, and reads the workspace back - proving it
 * survives a restart of the platform, not just a page reload.
 *
 * This suite duplicates a handful of small helpers from
 * `orchestration.test.ts` (free port allocation, starting/stopping a
 * built binary) rather than importing them: each file starts and stops
 * its own processes on its own schedule (this one restarts the platform
 * mid-test, which that one never does), and the shapes are a few lines
 * each - not worth a shared module for.
 *
 * The database is a file inside its own `mkdtempSync` directory, per
 * `docs/specs/workspaces.md` section 6 / W3: this suite's workspaces are
 * invisible to `orchestration.test.ts`'s suite (which never touches
 * `/api/workspaces` at all) and to a developer's own running platform,
 * because each gets its own temporary directory and `ORCHESTRA_DB_PATH`
 * has no default (`internal/infra/config.ErrMissingDBPath`) to fall back
 * to instead.
 */

interface RunningService {
  readonly process: ChildProcess;
  readonly port: number;
}

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

/** Starts the platform binary on a free port, over the given env, and waits for it to answer /api/health. */
async function startPlatform(env: Record<string, string>): Promise<RunningService> {
  const port = await freePort();
  const platformProcess = startBinary("../services/platform/bin/api", {
    ...env,
    ORCHESTRA_PORT: String(port),
  });

  await waitForReady(`http://127.0.0.1:${port}/api/health`, 10_000);

  return { process: platformProcess, port };
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
      // fine here, since this suite does not yet sign in (that is Task 6).
      ORCHESTRA_ADMIN_PASSWORD: "e2e-admin-password",
    };

    // Not pushed to `platforms`: the test stops it itself below, and
    // `stop`'s "exit" listener only fires for a process that has not
    // exited yet - attaching a second one in afterAll after that would
    // wait forever.
    const first = await startPlatform(sharedEnv);

    const createResponse = await fetch(`http://127.0.0.1:${first.port}/api/workspaces`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ name: "在庫ボード" }),
    });

    expect(createResponse.status).toBe(201);

    const created = parseWorkspaceCreated(await createResponse.json());

    const panelResponse = await fetch(
      `http://127.0.0.1:${first.port}/api/workspaces/${created.id}/panels`,
      {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          service: "inventory",
          operationId: "ListInventoryItems",
          args: {},
          component: "table",
          title: "検品保留の在庫",
        }),
      },
    );

    expect(panelResponse.status).toBe(201);

    // Stop the platform - the same process a restart or a deploy would
    // do - and start a fresh one on the same database file.
    await stop(first);

    const second = await startPlatform(sharedEnv);

    platforms.push(second);

    const readBack = await fetch(`http://127.0.0.1:${second.port}/api/workspaces/${created.id}`);

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
