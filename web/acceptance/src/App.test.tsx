import { App } from "@app-orchestra/web";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vite-plus/test";

/**
 * Application-level: renders the real App (AppBar + Drawer shell + chat
 * page) against a scripted platform, the way the built product will
 * actually run.
 */
describe("App", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("offers example questions and answers one against the platform", async () => {
    const user = userEvent.setup();

    vi.stubGlobal(
      "fetch",
      vi.fn(() =>
        Promise.resolve(
          new Response(
            JSON.stringify({
              kind: "result",
              component: "table",
              data: { items: [{ id: "itm-001", name: "ラベル用紙", status: "allocated" }] },
              source: { service: "inventory", operationId: "ListInventoryItems" },
            }),
            { status: 200, headers: { "content-type": "application/json" } },
          ),
        ),
      ),
    );

    render(<App />);

    expect(screen.getAllByText("チャット").length).toBeGreaterThan(0);

    const example = screen.getByRole("button", { name: "在庫の一覧を見せて" });
    await user.click(example);

    // The question becomes a turn, and the platform's answer follows it as a
    // table (Task 13) carrying its provenance. Detail/form/choice are Tasks
    // 14-15.
    expect(await screen.findByText("在庫の一覧を見せて")).toBeTruthy();
    expect(await screen.findByText("inventory / ListInventoryItems")).toBeTruthy();
    expect(await screen.findByText("itm-001")).toBeTruthy();
  });
});
