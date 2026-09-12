import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterAll, describe, expect, it } from "vite-plus/test";

import { signIn, withSession, type Session } from "./helpers/auth";
import { freePort, startBinary, stop, waitForReady, type RunningService } from "./helpers/process";

/**
 * Process-level end to end suite for a hand-built panel
 * (`docs/plans/dashboard.md` Task 7, Step 1) - the platform's own half of
 * "add a panel over a list operation with a group-by and a bar chart, see
 * it draw, reload, see it draw again": `GET /api/catalog`, a panel posted
 * with a `view` (a transform and a chart, both set by the caller, the way
 * `features/panels` would), and a read-back that proves the view
 * survived.
 *
 * No question is ever asked here and `/api/plan` is never called - P8's
 * whole point (`docs/specs/dashboard.md` section 2) is that a hand-built
 * panel never reaches the planner, so this suite has nothing to wire
 * `ORCHESTRA_PLAN_FIXTURES` for; the stub planner plays no part in it.
 *
 * `dashboard-permissions.test.ts` is this suite's sibling: AC-P-107's own
 * refusal, split out so this file stays under `max-lines`.
 *
 * Modelled on `workspaces.test.ts`: a built platform binary on a free
 * port, over its own temporary `ORCHESTRA_DB_PATH` file, signed in as
 * admin through `helpers/auth.ts` - every route but `GET /api/health` and
 * `POST /api/session` answers 401 without a session (AC-A-102).
 */

interface CatalogEntryOnWire {
  readonly service: string;
  readonly operationId: string;
  readonly component: string;
  readonly schema: Record<string, unknown>;
  readonly fields?: Record<string, unknown>;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isCatalogEntry(value: unknown): value is CatalogEntryOnWire {
  return (
    isRecord(value) &&
    typeof value["service"] === "string" &&
    typeof value["operationId"] === "string" &&
    typeof value["component"] === "string" &&
    isRecord(value["schema"])
  );
}

function parseCatalog(value: unknown): readonly CatalogEntryOnWire[] {
  if (!Array.isArray(value) || !value.every((entry) => isCatalogEntry(entry))) {
    throw new Error(`unexpected catalog response shape: ${JSON.stringify(value)}`);
  }

  return value;
}

/** The catalogue's own entry for inventory/ListInventoryItems, or throws - it must always be there. */
function requireListItemsEntry(catalog: readonly CatalogEntryOnWire[]): CatalogEntryOnWire {
  const entry = catalog.find((candidate) => isListInventoryItems(candidate));

  if (entry === undefined) {
    throw new Error("GET /api/catalog did not carry inventory/ListInventoryItems");
  }

  return entry;
}

function isListInventoryItems(entry: CatalogEntryOnWire): boolean {
  return entry.service === "inventory" && entry.operationId === "ListInventoryItems";
}

interface ViewOnWire {
  readonly transform?: {
    readonly groupBy: string;
    readonly aggregate: string;
    readonly field?: string;
  };
  readonly chart?: {
    readonly category: string;
    readonly value: string;
    readonly kind: string;
  };
}

interface PanelOnWire {
  readonly id: string;
  readonly service: string;
  readonly operationId: string;
  readonly component: string;
  readonly title: string;
  readonly view?: ViewOnWire;
}

function isPanel(value: unknown): value is PanelOnWire {
  return (
    isRecord(value) &&
    typeof value["id"] === "string" &&
    typeof value["service"] === "string" &&
    typeof value["operationId"] === "string" &&
    typeof value["component"] === "string"
  );
}

function parsePanel(value: unknown): PanelOnWire {
  if (!isPanel(value)) {
    throw new Error(`unexpected panel response shape: ${JSON.stringify(value)}`);
  }

  return value;
}

interface WorkspaceOnWire {
  readonly id: string;
  readonly name: string;
  readonly panels: readonly PanelOnWire[];
}

function isWorkspace(value: unknown): value is WorkspaceOnWire {
  return isRecord(value) && typeof value["id"] === "string" && Array.isArray(value["panels"]);
}

function parseWorkspace(value: unknown): WorkspaceOnWire {
  if (!isWorkspace(value)) {
    throw new Error(`unexpected workspace response shape: ${JSON.stringify(value)}`);
  }

  return value;
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

let inventory: RunningService | undefined;
let platform: RunningService | undefined;

afterAll(async () => {
  await Promise.all([stop(inventory), stop(platform)]);
});

describe("building a panel over a list operation, with a group-by and a bar chart", () => {
  it("carries its view through GET /api/catalog, POST a panel, and a read-back (AC-P-101, AC-P-102, AC-P-103, AC-P-104)", async () => {
    const inventoryPort = await freePort();

    inventory = {
      process: startBinary("../services/inventory/bin/api", {
        ORCHESTRA_PORT: String(inventoryPort),
      }),
      port: inventoryPort,
    };

    await waitForReady(`http://127.0.0.1:${inventoryPort}/openapi.yaml`, 10_000);

    const dbPath = join(mkdtempSync(join(tmpdir(), "orchestra-e2e-dashboard-")), "dashboard.db");
    const platformPort = await freePort();

    platform = {
      process: startBinary("../services/platform/bin/api", {
        ORCHESTRA_PORT: String(platformPort),
        ORCHESTRA_SERVICES: `inventory=http://127.0.0.1:${inventoryPort}`,
        ORCHESTRA_DB_PATH: dbPath,
        ORCHESTRA_ADMIN_PASSWORD: "e2e-admin-password",
        ORCHESTRA_SECURE_COOKIE: "false",
      }),
      port: platformPort,
    };

    await waitForReady(`http://127.0.0.1:${platformPort}/api/health`, 10_000);

    const session: Session = await signIn(
      `http://127.0.0.1:${platformPort}`,
      "admin",
      "e2e-admin-password",
    );

    // Step 1: GET /api/catalog - a person's own catalogue, no question
    // asked, from which a panel is built entirely in the browser
    // (docs/plans/dashboard.md Task 4, AC-P-101).
    const catalogResponse = await fetch(
      `http://127.0.0.1:${platformPort}/api/catalog`,
      withSession(session),
    );

    expect(catalogResponse.status).toBe(200);

    const catalog = parseCatalog(await catalogResponse.json());
    const listItems = requireListItemsEntry(catalog);

    expect(listItems.component).toBe("table");
    expect(listItems.fields?.["status"]).toBeDefined();

    // Step 1 (continued): a panel carrying a view - a transform (group by
    // status, count) and a chart (bar, over the same axes) - the same
    // shape `features/panels` posts, built here over bare HTTP instead of
    // through the browser (AC-P-102, AC-P-103, AC-P-104).
    const createWorkspace = await fetch(
      `http://127.0.0.1:${platformPort}/api/workspaces`,
      withSession(session, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ name: "在庫ダッシュボード" }),
      }),
    );

    expect(createWorkspace.status).toBe(201);

    const workspace = parseWorkspaceCreated(await createWorkspace.json());

    const view = {
      transform: { groupBy: "status", aggregate: "count" },
      chart: { category: "status", value: "count", kind: "bar" },
    };

    const createPanel = await fetch(
      `http://127.0.0.1:${platformPort}/api/workspaces/${workspace.id}/panels`,
      withSession(session, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          service: "inventory",
          operationId: "ListInventoryItems",
          args: {},
          component: "chart",
          title: "ステータス別の件数",
          view,
        }),
      }),
    );

    expect(createPanel.status).toBe(201);

    const created = parsePanel(await createPanel.json());

    expect(created.view).toEqual(view);

    // Step 1 (continued): a read-back proves the view survived the round
    // trip through the store, the same way a page reload would draw it
    // again from the saved panel rather than from anything still held in
    // memory (AC-P-104, AC-P-106).
    const readBack = await fetch(
      `http://127.0.0.1:${platformPort}/api/workspaces/${workspace.id}`,
      withSession(session),
    );

    expect(readBack.status).toBe(200);

    const reloaded = parseWorkspace(await readBack.json());

    expect(reloaded.panels).toHaveLength(1);
    const [panel] = reloaded.panels;

    expect(panel?.component).toBe("chart");
    expect(panel?.view).toEqual(view);
  }, 30_000);
});
