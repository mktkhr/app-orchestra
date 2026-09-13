import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vite-plus/test";

import type { getCatalog } from "@/shared/api/catalog";
import { postInvoke, type WorkspacePanel } from "@/shared/api/client";

import { PanelResult } from "./PanelResult";

vi.mock("@/shared/api/client", () => ({
  postInvoke: vi.fn<typeof postInvoke>(),
}));

// The default catalogue is empty: `useCatalogEntry` finds no entry for any
// panel, which `PanelResult` reads as safe-to-invoke, so every test below
// behaves exactly as before defect 1's fix (docs/specs/dashboard.md P14).
// The unsafe-operation behaviour has its own suite,
// `PanelResultUnsafeOperation.test.tsx` (split out for `max-lines`).
vi.mock("@/shared/api/catalog", () => ({
  getCatalog: vi.fn<typeof getCatalog>().mockResolvedValue([]),
}));

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

describe("PanelResult", () => {
  it("draws a table result with the panel's title and provenance", async () => {
    vi.mocked(postInvoke).mockResolvedValue({
      component: "table",
      data: { items: [{ id: "itm-001", name: "品目1" }] },
    });

    render(<PanelResult workspaceId="ws-1" panel={panel()} />);

    expect(screen.getByText("検品保留の在庫")).toBeTruthy();
    expect(await screen.findByText("itm-001")).toBeTruthy();
    expect(screen.getByText("inventory / ListInventoryItems")).toBeTruthy();
    expect(postInvoke).toHaveBeenCalledWith({
      service: "inventory",
      operationId: "ListInventoryItems",
      args: { status: "quarantined" },
    });
  });

  it("shows the failure inside its own card, without throwing", async () => {
    vi.mocked(postInvoke).mockRejectedValue(new Error("boom"));

    render(
      <PanelResult
        workspaceId="ws-1"
        panel={panel({ id: "pnl-2", title: "接続できないパネル" })}
      />,
    );

    expect(await screen.findByText(/取得に失敗しました/u)).toBeTruthy();
    expect(screen.getByText("接続できないパネル")).toBeTruthy();
  });

  it("loads independently: a pending panel does not block another panel's result", async () => {
    let resolveSlow: ((value: Awaited<ReturnType<typeof postInvoke>>) => void) | undefined;

    vi.mocked(postInvoke)
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolveSlow = resolve;
          }),
      )
      .mockResolvedValueOnce({ component: "table", data: { items: [{ id: "itm-fast" }] } });

    render(
      <>
        <PanelResult
          workspaceId="ws-1"
          panel={panel({ id: "pnl-slow", operationId: "Slow", title: "遅いパネル" })}
        />
        <PanelResult
          workspaceId="ws-1"
          panel={panel({ id: "pnl-fast", operationId: "Fast", title: "速いパネル" })}
        />
      </>,
    );

    expect(await screen.findByText("itm-fast")).toBeTruthy();
    expect(resolveSlow).toBeDefined();
  });

  it("runs the panel again and draws a row the first call did not have (W2)", async () => {
    vi.mocked(postInvoke)
      .mockResolvedValueOnce({ component: "table", data: { items: [{ id: "itm-001" }] } })
      .mockResolvedValueOnce({
        component: "table",
        data: { items: [{ id: "itm-001" }, { id: "itm-new" }] },
      });

    render(<PanelResult workspaceId="ws-1" panel={panel()} />);
    expect(await screen.findByText("itm-001")).toBeTruthy();
    expect(screen.queryByText("itm-new")).toBeFalsy();
    vi.mocked(postInvoke).mockClear();

    fireEvent.click(screen.getByRole("button", { name: "更新" }));

    expect(await screen.findByText("itm-new")).toBeTruthy();
    expect(postInvoke).toHaveBeenCalledTimes(1);
  });

  it("keeps the previous result on screen while a refresh is in flight", async () => {
    let resolveRefresh: ((value: Awaited<ReturnType<typeof postInvoke>>) => void) | undefined;

    vi.mocked(postInvoke)
      .mockResolvedValueOnce({ component: "table", data: { items: [{ id: "itm-001" }] } })
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolveRefresh = resolve;
          }),
      );

    render(<PanelResult workspaceId="ws-1" panel={panel()} />);
    expect(await screen.findByText("itm-001")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "更新" }));

    expect(await screen.findByRole("button", { name: "更新中" })).toBeTruthy();
    expect(screen.getByText("itm-001")).toBeTruthy();

    resolveRefresh?.({ component: "table", data: { items: [{ id: "itm-001" }] } });
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "更新" })).toBeTruthy();
    });
  });

  it("keeps the previous result and shows the error when a refresh fails", async () => {
    vi.mocked(postInvoke)
      .mockResolvedValueOnce({ component: "table", data: { items: [{ id: "itm-001" }] } })
      .mockRejectedValueOnce(new Error("boom"));

    render(<PanelResult workspaceId="ws-1" panel={panel()} />);
    expect(await screen.findByText("itm-001")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "更新" }));

    expect(await screen.findByText(/取得に失敗しました/u)).toBeTruthy();
    expect(screen.getByText("itm-001")).toBeTruthy();
  });

  it("draws a chart when the panel's view names one (AC-P-103/104)", async () => {
    vi.mocked(postInvoke).mockResolvedValue({
      component: "table",
      data: { items: [{ status: "allocated", count: 3 }] },
    });

    const { container } = render(
      <PanelResult
        workspaceId="ws-1"
        panel={panel({
          view: { chart: { category: "status", value: "count", kind: "bar" } },
        })}
      />,
    );

    await waitFor(() => {
      expect(container.querySelectorAll(".MuiBarChart-element")).toHaveLength(1);
    });
    // A chart draws its own figcaption; it does not also draw a table alongside it.
    expect(container.querySelectorAll("table")).toHaveLength(0);
  });

  it("draws the grouped rows, not the raw ones, when the panel's view names a transform", async () => {
    vi.mocked(postInvoke).mockResolvedValue({
      component: "table",
      data: {
        items: [
          { status: "allocated", count: 1 },
          { status: "allocated", count: 1 },
          { status: "staged", count: 1 },
        ],
      },
    });

    render(
      <PanelResult
        workspaceId="ws-1"
        panel={panel({
          view: { transform: { groupBy: "status", aggregate: "count" } },
        })}
      />,
    );

    expect(await screen.findByText("allocated")).toBeTruthy();
    expect(screen.getByText("staged")).toBeTruthy();
    // Two source rows became one "allocated" group of count 2, not two rows.
    expect(screen.getAllByText("2")).toHaveLength(1);
  });

  it("draws a chart of the grouped rows when the panel's view names both", async () => {
    vi.mocked(postInvoke).mockResolvedValue({
      component: "table",
      data: {
        items: [
          { status: "allocated", count: 1 },
          { status: "allocated", count: 1 },
          { status: "staged", count: 1 },
        ],
      },
    });

    const { container } = render(
      <PanelResult
        workspaceId="ws-1"
        panel={panel({
          view: {
            transform: { groupBy: "status", aggregate: "count" },
            chart: { category: "status", value: "count", kind: "bar" },
          },
        })}
      />,
    );

    // Grouped into two bars ("allocated", "staged"), not the three raw rows.
    await waitFor(() => {
      expect(container.querySelectorAll(".MuiBarChart-element")).toHaveLength(2);
    });
  });

  it("draws exactly what it draws today when the panel carries no view at all (AC-P-106)", async () => {
    vi.mocked(postInvoke).mockResolvedValue({
      component: "table",
      data: { items: [{ id: "itm-001", name: "品目1" }] },
    });

    const { container } = render(<PanelResult workspaceId="ws-1" panel={panel()} />);

    expect(await screen.findByText("itm-001")).toBeTruthy();
    expect(container.querySelectorAll(".MuiBarChart-element")).toHaveLength(0);
  });

  it("draws the chart's own empty state when its rows carry none of its fields", async () => {
    vi.mocked(postInvoke).mockResolvedValue({
      component: "table",
      data: { items: [{ id: "itm-001" }] },
    });

    render(
      <PanelResult
        workspaceId="ws-1"
        panel={panel({
          view: { chart: { category: "status", value: "count", kind: "bar" } },
        })}
      />,
    );

    expect(await screen.findByText("結果は0件です。")).toBeTruthy();
  });

  it("ignores a second click while a refresh is already in flight", async () => {
    let resolveRefresh: ((value: Awaited<ReturnType<typeof postInvoke>>) => void) | undefined;

    vi.mocked(postInvoke)
      .mockResolvedValueOnce({ component: "table", data: { items: [{ id: "itm-001" }] } })
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolveRefresh = resolve;
          }),
      );

    render(<PanelResult workspaceId="ws-1" panel={panel()} />);
    expect(await screen.findByText("itm-001")).toBeTruthy();
    vi.mocked(postInvoke).mockClear();

    const button = screen.getByRole("button", { name: "更新" });
    fireEvent.click(button);
    fireEvent.click(button);
    fireEvent.click(button);

    await screen.findByRole("button", { name: "更新中" });
    expect(postInvoke).toHaveBeenCalledTimes(1);

    resolveRefresh?.({ component: "table", data: { items: [{ id: "itm-001" }] } });
    // Settles the refresh fully before the test ends: an unawaited resolve
    // here left a pending state update to land during whichever test ran
    // next, inflating that test's own `postInvoke` count.
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "更新" })).toBeTruthy();
    });
  });
});
