import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vite-plus/test";

import { postInvoke, type WorkspacePanel } from "@/shared/api/client";

import { PanelResult } from "./PanelResult";

vi.mock("@/shared/api/client", () => ({
  postInvoke: vi.fn<typeof postInvoke>(),
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
    ...overrides,
  };
}

describe("PanelResult", () => {
  it("draws a table result with the panel's title and provenance", async () => {
    vi.mocked(postInvoke).mockResolvedValue({
      component: "table",
      data: { items: [{ id: "itm-001", name: "品目1" }] },
    });

    render(<PanelResult panel={panel()} />);

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

    render(<PanelResult panel={panel({ id: "pnl-2", title: "接続できないパネル" })} />);

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
        <PanelResult panel={panel({ id: "pnl-slow", operationId: "Slow", title: "遅いパネル" })} />
        <PanelResult panel={panel({ id: "pnl-fast", operationId: "Fast", title: "速いパネル" })} />
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

    render(<PanelResult panel={panel()} />);
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

    render(<PanelResult panel={panel()} />);
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

    render(<PanelResult panel={panel()} />);
    expect(await screen.findByText("itm-001")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "更新" }));

    expect(await screen.findByText(/取得に失敗しました/u)).toBeTruthy();
    expect(screen.getByText("itm-001")).toBeTruthy();
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

    render(<PanelResult panel={panel()} />);
    expect(await screen.findByText("itm-001")).toBeTruthy();
    vi.mocked(postInvoke).mockClear();

    const button = screen.getByRole("button", { name: "更新" });
    fireEvent.click(button);
    fireEvent.click(button);
    fireEvent.click(button);

    await screen.findByRole("button", { name: "更新中" });
    expect(postInvoke).toHaveBeenCalledTimes(1);

    resolveRefresh?.({ component: "table", data: { items: [{ id: "itm-001" }] } });
  });
});
