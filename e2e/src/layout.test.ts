import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterAll, describe, expect, it } from "vite-plus/test";

import { signIn, withSession, type Session } from "./helpers/auth";
import { freePort, startBinary, stop, waitForReady, type RunningService } from "./helpers/process";

/**
 * Process-level end to end suite for arranging a workspace
 * (`docs/plans/layout.md` Task 3, Step 1; AC-L-101, AC-L-102, AC-L-104):
 * three panels are created, one is made wide and moved first through the
 * same `PATCH` `useArrangement` calls, and a fresh read of the workspace
 * shows the geometry that request wrote rather than the order the panels
 * were created in.
 *
 * Modelled on `workspaces.test.ts`: a built platform binary on a free
 * port, over its own temporary `ORCHESTRA_DB_PATH` file, signed in as
 * admin through `helpers/auth.ts`. Unlike that suite, the platform here is
 * never restarted - the claim under test is that a `PATCH` a browser sent
 * is what a later `GET` reads back, not that the value survives the
 * platform's own process. `dashboard.test.ts` proves a panel's `view`
 * survives the same way, over the same kind of `GET`.
 *
 * AC-L-103 (the keyboard path) and AC-L-105/AC-L-106 (the grid's own
 * layout and a chart's own size) are not process-level claims - nothing
 * here renders a page - and are covered instead by
 * `e2e/browser/layout.spec.ts` and `harness/quality/browser/*` (see this
 * task's own report in `DECISIONS.md`).
 */

interface PanelOnWire {
  readonly id: string;
  readonly position: number;
  readonly width: number;
  readonly height: number;
}

interface WorkspaceOnWire {
  readonly id: string;
  readonly panels: readonly PanelOnWire[];
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isPanelOnWire(value: unknown): value is PanelOnWire {
  return (
    isRecord(value) &&
    typeof value["id"] === "string" &&
    typeof value["position"] === "number" &&
    typeof value["width"] === "number" &&
    typeof value["height"] === "number"
  );
}

function isWorkspaceOnWire(value: unknown): value is WorkspaceOnWire {
  return (
    isRecord(value) &&
    typeof value["id"] === "string" &&
    Array.isArray(value["panels"]) &&
    value["panels"].every((panel) => isPanelOnWire(panel))
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
}

function isWorkspaceCreatedOnWire(value: unknown): value is WorkspaceCreatedOnWire {
  return isRecord(value) && typeof value["id"] === "string";
}

function parseWorkspaceCreated(value: unknown): WorkspaceCreatedOnWire {
  if (!isWorkspaceCreatedOnWire(value)) {
    throw new Error(`unexpected workspace-created response shape: ${JSON.stringify(value)}`);
  }

  return value;
}

interface PanelCreatedOnWire {
  readonly id: string;
}

function isPanelCreatedOnWire(value: unknown): value is PanelCreatedOnWire {
  return isRecord(value) && typeof value["id"] === "string";
}

function parsePanelCreated(value: unknown): PanelCreatedOnWire {
  if (!isPanelCreatedOnWire(value)) {
    throw new Error(`unexpected panel-created response shape: ${JSON.stringify(value)}`);
  }

  return value;
}

/** Three distinct panel ids, or throws - kept out of the test body itself so a missing one is a setup failure, not a branch inside it. */
function requireThreeIds(ids: readonly string[]): readonly [string, string, string] {
  const [first, second, third] = ids;

  if (first === undefined || second === undefined || third === undefined) {
    throw new Error(`expected three panel ids, got ${String(ids.length)}`);
  }

  return [first, second, third];
}

let inventory: RunningService | undefined;
let platform: RunningService | undefined;

afterAll(async () => {
  await Promise.all([stop(inventory), stop(platform)]);
});

describe("arranging a workspace (AC-L-101, AC-L-102, AC-L-104)", () => {
  it("saves a wider panel moved first, and reads it back that way", async () => {
    const inventoryPort = await freePort();

    inventory = {
      process: startBinary("../services/inventory/bin/api", {
        ORCHESTRA_PORT: String(inventoryPort),
      }),
      port: inventoryPort,
    };

    await waitForReady(`http://127.0.0.1:${inventoryPort}/openapi.yaml`, 10_000);

    const dbPath = join(mkdtempSync(join(tmpdir(), "orchestra-e2e-layout-")), "layout.db");
    const platformPort = await freePort();

    platform = {
      process: startBinary("../services/platform/bin/api", {
        ORCHESTRA_PORT: String(platformPort),
        ORCHESTRA_SERVICES: `inventory=http://127.0.0.1:${inventoryPort}`,
        ORCHESTRA_DB_PATH: dbPath,
        ORCHESTRA_ADMIN_PASSWORD: "e2e-admin-password",
        // Plain HTTP (127.0.0.1, no TLS) - see orchestration.test.ts's own comment.
        ORCHESTRA_SECURE_COOKIE: "false",
      }),
      port: platformPort,
    };

    const baseUrl = `http://127.0.0.1:${platformPort}`;

    await waitForReady(`${baseUrl}/api/health`, 10_000);

    const session: Session = await signIn(baseUrl, "admin", "e2e-admin-password");

    const workspaceResponse = await fetch(
      `${baseUrl}/api/workspaces`,
      withSession(session, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ name: "並べ替えワークスペース" }),
      }),
    );

    expect(workspaceResponse.status).toBe(201);

    const workspace = parseWorkspaceCreated(await workspaceResponse.json());

    // Three panels over the same, real operation - only their geometry is
    // under test here, not what they call.
    const panelIds: string[] = [];

    for (let index = 0; index < 3; index += 1) {
      const panelResponse = await fetch(
        `${baseUrl}/api/workspaces/${workspace.id}/panels`,
        withSession(session, {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({
            service: "inventory",
            operationId: "ListInventoryItems",
            args: {},
            component: "table",
            title: `パネル${String(index)}`,
          }),
        }),
      );

      expect(panelResponse.status).toBe(201);

      panelIds.push(parsePanelCreated(await panelResponse.json()).id);
    }

    const [first, second, third] = requireThreeIds(panelIds);

    // `addPanel` never assigns a panel a position of its own (every panel
    // above was created with none named, so all three still carry the
    // domain's zero value) - a workspace looks arranged only once
    // something actually arranges it (this task's own "Done" line), which
    // here is these three `PATCH`es: first and second take the two rows
    // after the first, and third is both widened to a full-width, two-row
    // span (AC-L-101) and moved to the front (AC-L-102) - only its own row,
    // and the ones it displaces, change (docs/specs/layout.md section 6).
    const patches: readonly [string, Record<string, unknown>][] = [
      [first, { position: 1 }],
      [second, { position: 2 }],
      [third, { width: 12, height: 2, position: 0 }],
    ];

    for (const [panelId, body] of patches) {
      const patchResponse = await fetch(
        `${baseUrl}/api/workspaces/${workspace.id}/panels/${panelId}`,
        withSession(session, {
          method: "PATCH",
          headers: { "content-type": "application/json" },
          body: JSON.stringify(body),
        }),
      );

      expect(patchResponse.status).toBe(200);
    }

    // A fresh GET, not anything held over from the PATCH responses above -
    // the geometry has to have actually been written, not merely echoed.
    const readBack = await fetch(`${baseUrl}/api/workspaces/${workspace.id}`, withSession(session));

    expect(readBack.status).toBe(200);

    const reloaded = parseWorkspace(await readBack.json());
    const byId = new Map(reloaded.panels.map((panel) => [panel.id, panel]));

    expect(byId.get(third)).toMatchObject({ position: 0, width: 12, height: 2 });
    expect(byId.get(first)).toMatchObject({ position: 1, width: 12, height: 1 });
    // `second`'s width and height were never named in its own `PATCH`
    // above - only `position` was - so they read back exactly as
    // `addPanel` defaulted them (AC-L-104): a field a request does not
    // name is a field that request leaves alone.
    expect(byId.get(second)).toMatchObject({ position: 2, width: 12, height: 1 });
  }, 30_000);
});
