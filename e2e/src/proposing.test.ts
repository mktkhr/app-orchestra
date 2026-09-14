import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterAll, beforeAll, describe, expect, it } from "vite-plus/test";

import { signIn, withSession, type Session } from "./helpers/auth";
import { freePort, startBinary, stop, waitForReady, type RunningService } from "./helpers/process";

/**
 * Process-level end to end suite for "asking for a panel"
 * (`docs/plans/proposing.md`, Task 2; `docs/specs/proposing.md`).
 *
 * Asks a question over real TCP against `/api/plan` and gets back
 * `kind: "proposal"`, then proves the workspace is unchanged until that
 * proposal is placed through `POST /api/workspaces/{id}/panels`, the same
 * endpoint a hand-built panel already uses (`e2e/src/dashboard.test.ts`).
 * `e2e/browser/proposing.spec.ts` proves the same product through a
 * browser.
 *
 * No real LLM: the stub planner answers over `ORCHESTRA_PLAN_FIXTURES`,
 * fed a `propose: true` fixture - see `internal/infra/config.PlanFixture`
 * and `pkg/app.PlanFixture` for the plumbing this exercises (AC-N-106).
 */

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

interface ProposedPanelOnWire {
  readonly service: string;
  readonly operationId: string;
  readonly args: Record<string, unknown>;
  readonly component: string;
  readonly title: string;
  readonly view?: ViewOnWire;
}

interface ProposalResponseBody {
  readonly kind: string;
  readonly panel: ProposedPanelOnWire;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isProposedPanel(value: unknown): value is ProposedPanelOnWire {
  return (
    isRecord(value) &&
    typeof value["service"] === "string" &&
    typeof value["operationId"] === "string" &&
    isRecord(value["args"]) &&
    typeof value["component"] === "string" &&
    typeof value["title"] === "string"
  );
}

function isProposalResponse(value: unknown): value is ProposalResponseBody {
  return isRecord(value) && typeof value["kind"] === "string" && isProposedPanel(value["panel"]);
}

/** Parses a POST /api/plan proposal body without an unsafe type assertion. */
function parseProposalResponse(value: unknown): ProposalResponseBody {
  if (!isProposalResponse(value)) {
    throw new Error(`unexpected /api/plan proposal response shape: ${JSON.stringify(value)}`);
  }

  return value;
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

const proposeQuery = "在庫をステータス別に棒グラフで置いて";

let inventory: RunningService | undefined;
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

beforeAll(async () => {
  const inventoryPort = await freePort();

  inventory = {
    process: startBinary("../services/inventory/bin/api", {
      ORCHESTRA_PORT: String(inventoryPort),
    }),
    port: inventoryPort,
  };

  await waitForReady(`http://127.0.0.1:${inventoryPort}/openapi.yaml`, 10_000);
  const planFixtures = [
    {
      query: proposeQuery,
      propose: true,
      service: "inventory",
      operationId: "ListInventoryItems",
      args: {},
      component: "chart",
      title: "ステータス別の在庫",
      chart: { category: "status", value: "count", kind: "bar" },
    },
  ];

  // Own temp dir (docs/specs/workspaces.md section 6): no cross-suite workspaces.
  const dbPath = join(mkdtempSync(join(tmpdir(), "orchestra-e2e-proposing-")), "workspaces.db");
  const platformPort = await freePort();

  platform = {
    process: startBinary("../services/platform/bin/api", {
      ORCHESTRA_PORT: String(platformPort),
      ORCHESTRA_SERVICES: `inventory=http://127.0.0.1:${inventoryPort}`,
      ORCHESTRA_PLAN_FIXTURES: JSON.stringify(planFixtures),
      ORCHESTRA_DB_PATH: dbPath,
      ORCHESTRA_ADMIN_PASSWORD: "e2e-admin-password",
      // Plain HTTP (127.0.0.1, no TLS): see orchestration.test.ts.
      ORCHESTRA_SECURE_COOKIE: "false",
    }),
    port: platformPort,
  };

  await waitForReady(`http://127.0.0.1:${platformPort}/api/health`, 10_000);

  session = await signIn(`http://127.0.0.1:${platformPort}`, "admin", "e2e-admin-password");
}, 30_000);

afterAll(async () => {
  await Promise.all([stop(inventory), stop(platform)]);
});

describe("asking a workspace's chat for a panel (AC-N-101, AC-N-102, AC-N-106)", () => {
  it("proposes a panel naming the operation, args and view the question described, writes nothing until placed, and is placed as the person accepted it", async () => {
    const port = requirePlatform().port;

    // A workspace to place the accepted panel into (N4/AC-N-104 - only the
    // workspace screen's chat offers a proposal - is covered at the
    // component level, not here).
    const createWorkspace = await fetch(`http://127.0.0.1:${port}/api/workspaces`, {
      ...withSession(requireSession(), {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ name: "在庫ダッシュボード" }),
      }),
    });

    expect(createWorkspace.status).toBe(201);

    const workspace = parseWorkspaceCreated(await createWorkspace.json());

    // workspaceId required: propose_panel is offered only from a workspace
    // (docs/specs/offering.md, O3/O4).
    const planResponse = await fetch(`http://127.0.0.1:${port}/api/plan`, {
      ...withSession(requireSession(), {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ query: proposeQuery, workspaceId: workspace.id }),
      }),
    });

    expect(planResponse.status).toBe(200);

    const proposal = parseProposalResponse(await planResponse.json());

    // AC-N-101: operation, arguments and view, as the question described.
    expect(proposal.kind).toBe("proposal");
    expect(proposal.panel).toEqual({
      service: "inventory",
      operationId: "ListInventoryItems",
      args: {},
      component: "chart",
      title: "ステータス別の在庫",
      view: { chart: { category: "status", value: "count", kind: "bar" } },
    });

    // AC-N-102 (first half): asking wrote nothing - no panel exists yet.
    const beforePlacing = await fetch(
      `http://127.0.0.1:${port}/api/workspaces/${workspace.id}`,
      withSession(requireSession()),
    );

    expect(beforePlacing.status).toBe(200);
    expect(parseWorkspace(await beforePlacing.json()).panels).toHaveLength(0);

    // Placing it: the same endpoint a hand-built panel uses; the model
    // plays no part in this request.
    const placePanel = await fetch(
      `http://127.0.0.1:${port}/api/workspaces/${workspace.id}/panels`,
      withSession(requireSession(), {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(proposal.panel),
      }),
    );

    expect(placePanel.status).toBe(201);

    const placed = parsePanel(await placePanel.json());

    expect(placed.component).toBe("chart");
    expect(placed.title).toBe("ステータス別の在庫");
    expect(placed.view).toEqual({ chart: { category: "status", value: "count", kind: "bar" } });

    // AC-N-102 (second half): the workspace now holds the placed panel.
    const afterPlacing = await fetch(
      `http://127.0.0.1:${port}/api/workspaces/${workspace.id}`,
      withSession(requireSession()),
    );

    expect(afterPlacing.status).toBe(200);

    const reloaded = parseWorkspace(await afterPlacing.json());

    expect(reloaded.panels).toHaveLength(1);
    expect(reloaded.panels[0]?.id).toBe(placed.id);
  }, 30_000);
});
