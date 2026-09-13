import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vite-plus/test";

import { getCatalog, type CatalogEntry } from "@/shared/api/catalog";
import { postInvoke, type WorkspacePanel } from "@/shared/api/client";

import { PanelResult } from "./PanelResult";

// Split out of PanelResult.test.tsx only for eslint's `max-lines` (300) -
// the panel-over-an-unsafe-operation behaviour (`docs/specs/dashboard.md`
// P14, section 6b) is its own concern, not a second suite for the same
// component.
vi.mock("@/shared/api/client", () => ({
  postInvoke: vi.fn<typeof postInvoke>(),
}));

// The default catalogue is empty: `useCatalogEntry` then finds no entry for
// any panel, which `PanelResult` reads exactly as it reads a catalogue
// fetch failure - safe-to-invoke. Every test here overrides it, per test,
// to a catalogue that actually lists the operation under test.
vi.mock("@/shared/api/catalog", () => ({
  getCatalog: vi.fn<typeof getCatalog>().mockResolvedValue([]),
}));

function catalogEntry(overrides: Partial<CatalogEntry> = {}): CatalogEntry {
  return {
    service: "inventory",
    serviceDisplayName: "在庫管理",
    operationId: "CreateInventoryItem",
    summary: "Create an inventory item",
    displayName: "在庫を登録",
    component: "form",
    schema: {
      type: "object",
      required: ["name"],
      properties: { name: { type: "string", title: "品目名" } },
    },
    ...overrides,
  };
}

function panel(overrides: Partial<WorkspacePanel> = {}): WorkspacePanel {
  return {
    id: "pnl-1",
    workspaceId: "ws-1",
    service: "inventory",
    operationId: "ListInventoryItems",
    args: { status: "quarantined" },
    component: "table",
    title: "検品保留の在庫",
    position: 0,
    width: 12,
    height: 1,
    ...overrides,
  };
}

describe("PanelResult, over an unsafe operation (docs/specs/dashboard.md P14, AC-P-111)", () => {
  it("stays safe when the catalogue lists this exact operation as a table (defect 1 regression guard)", async () => {
    vi.mocked(getCatalog).mockResolvedValueOnce([
      catalogEntry({ operationId: "ListInventoryItems", component: "table", schema: {} }),
    ]);
    vi.mocked(postInvoke).mockResolvedValue({
      component: "table",
      data: { items: [{ id: "itm-001" }] },
    });
    // A count-sensitive assertion, so it starts from a clean slate the same
    // way every other such assertion in this file does - restoreMocks
    // resets each mock's own configuration before a test, not a call
    // recorded mid-test by a still-settling previous one.
    vi.mocked(postInvoke).mockClear();

    render(<PanelResult workspaceId="ws-1" panel={panel()} />);

    expect(await screen.findByText("itm-001")).toBeTruthy();
    expect(postInvoke).toHaveBeenCalledTimes(1);
  });

  function unsafePanel(overrides: Partial<WorkspacePanel> = {}): WorkspacePanel {
    return panel({
      operationId: "CreateInventoryItem",
      args: { name: "初期値" },
      component: "form",
      title: "在庫を登録",
      ...overrides,
    });
  }

  it("draws the operation's form and calls /api/invoke zero times on mount", async () => {
    vi.mocked(getCatalog).mockResolvedValueOnce([catalogEntry()]);
    vi.mocked(postInvoke).mockClear();

    render(<PanelResult workspaceId="ws-1" panel={unsafePanel()} />);

    expect(await screen.findByRole("button", { name: "送信" })).toBeTruthy();
    expect(screen.getByRole("textbox", { name: "品目名" })).toHaveProperty("value", "初期値");
    expect(postInvoke).not.toHaveBeenCalled();
  });

  it("still calls nothing when the panel's refresh control is pressed", async () => {
    vi.mocked(getCatalog).mockResolvedValueOnce([catalogEntry()]);
    vi.mocked(postInvoke).mockClear();

    render(<PanelResult workspaceId="ws-1" panel={unsafePanel()} />);
    await screen.findByRole("button", { name: "送信" });

    fireEvent.click(screen.getByRole("button", { name: "更新" }));

    expect(await screen.findByRole("button", { name: "送信" })).toBeTruthy();
    expect(postInvoke).not.toHaveBeenCalled();
  });

  it("calls /api/invoke exactly once when the form is submitted, and then draws the result", async () => {
    vi.mocked(getCatalog).mockResolvedValueOnce([catalogEntry()]);
    vi.mocked(postInvoke).mockResolvedValue({
      component: "detail",
      data: { id: "itm-999", name: "初期値" },
    });
    vi.mocked(postInvoke).mockClear();

    render(<PanelResult workspaceId="ws-1" panel={unsafePanel()} />);
    const submit = await screen.findByRole("button", { name: "送信" });

    fireEvent.click(submit);

    expect(await screen.findByText("itm-999")).toBeTruthy();
    expect(postInvoke).toHaveBeenCalledTimes(1);
    expect(postInvoke).toHaveBeenCalledWith({
      service: "inventory",
      operationId: "CreateInventoryItem",
      args: { name: "初期値" },
    });
  });
});
