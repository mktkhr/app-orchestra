import { App } from "@app-orchestra/web";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vite-plus/test";

/**
 * Answers GET /api/session with a signed-in account (docs/plans/auth.md
 * Task 4) so the shell renders instead of the sign-in screen, and answers
 * everything else with the same scripted plan result the test exercises.
 */
function respond(input: RequestInfo | URL): Promise<Response> {
  const url = input instanceof Request ? input.url : String(input);

  if (url.endsWith("/api/session")) {
    return Promise.resolve(
      new Response(JSON.stringify({ id: "usr-1", name: "admin", role: "admin" }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
  }

  return Promise.resolve(
    new Response(
      JSON.stringify({
        kind: "result",
        component: "table",
        data: { items: [{ id: "itm-001", name: "ラベル用紙", status: "allocated" }] },
        source: {
          service: "inventory",
          serviceDisplayName: "在庫管理",
          operationId: "ListInventoryItems",
        },
      }),
      { status: 200, headers: { "content-type": "application/json" } },
    ),
  );
}

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

    vi.stubGlobal("fetch", vi.fn(respond));

    render(<App />);

    expect((await screen.findAllByText("チャット")).length).toBeGreaterThan(0);
    expect(await screen.findByText("admin")).toBeTruthy();

    const example = screen.getByRole("button", { name: "在庫の一覧を見せて" });
    await user.click(example);

    // The question becomes a turn, and the platform's answer follows it as a
    // table (Task 13) carrying its provenance. Detail/form/choice are Tasks
    // 14-15.
    expect(await screen.findByText("在庫の一覧を見せて")).toBeTruthy();
    expect(await screen.findByText("在庫管理 / ListInventoryItems")).toBeTruthy();
    expect(await screen.findByText("itm-001")).toBeTruthy();
  });
});
