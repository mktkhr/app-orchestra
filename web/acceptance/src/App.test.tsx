import { App } from "@app-orchestra/web";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vite-plus/test";

/** Every `/api/plan` request body this test's `respond` has answered, in order. */
const planRequests: unknown[] = [];

/**
 * Answers GET /api/session with a signed-in account (docs/plans/auth.md
 * Task 4) so the shell renders instead of the sign-in screen, and answers
 * `/api/plan` with a table result carrying two alternatives
 * (docs/specs/shortlisting.md, section 4) - unless the request itself
 * carries `preferred`, the shape a chosen alternative re-asks with, in
 * which case it answers with `kind: "none"` instead, so a test can tell
 * the two calls apart by their answer alone.
 */
async function respond(input: RequestInfo | URL): Promise<Response> {
  const url = input instanceof Request ? input.url : String(input);

  if (url.endsWith("/api/session")) {
    return new Response(JSON.stringify({ id: "usr-1", name: "admin", role: "admin" }), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  }

  if (!url.endsWith("/api/plan")) {
    return new Response(JSON.stringify([]), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  }

  if (!(input instanceof Request)) {
    throw new Error("expected POST /api/plan to be called with a Request");
  }

  // openapi-fetch calls `fetch(new Request(url, init))` - the body sits on
  // the `Request` itself, not on a separate `init`.
  const body: unknown = JSON.parse(await input.clone().text());
  planRequests.push(body);

  if (typeof body === "object" && body !== null && "preferred" in body) {
    return new Response(JSON.stringify({ kind: "none", message: "結果はありません。" }), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  }

  return new Response(
    JSON.stringify({
      kind: "result",
      component: "table",
      data: { items: [{ id: "itm-001", name: "ラベル用紙", status: "allocated" }] },
      source: {
        service: "inventory",
        serviceDisplayName: "在庫管理",
        operationId: "ListInventoryItems",
      },
      alternatives: [
        { operationId: "createInventoryItem", displayName: "在庫を登録", service: "inventory" },
      ],
    }),
    { status: 200, headers: { "content-type": "application/json" } },
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
    planRequests.length = 0;
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

  // docs/specs/shortlisting.md, section 4 (H5): the result's alternatives
  // draw as chips under it, and choosing one re-asks the same question with
  // that alternative's operationId as `preferred`.
  it("offers alternatives under a result, and re-plans with the one chosen", async () => {
    const user = userEvent.setup();

    vi.stubGlobal("fetch", vi.fn(respond));

    render(<App />);

    const example = await screen.findByRole("button", { name: "在庫の一覧を見せて" });
    await user.click(example);
    await screen.findByText("itm-001");

    expect(await screen.findByText("違いましたか？")).toBeTruthy();
    const chip = screen.getByText("在庫を登録");
    await user.click(chip);

    expect(await screen.findByText("結果はありません。")).toBeTruthy();
    expect(planRequests.at(-1)).toEqual(
      expect.objectContaining({
        query: "在庫の一覧を見せて",
        preferred: "createInventoryItem",
      }),
    );

    // The chip click reads as a choice, not the question being sent again
    // (docs/specs/shortlisting.md, section 4, H5).
    expect(await screen.findByText("→ 在庫を登録")).toBeTruthy();
    expect(screen.getAllByText("在庫の一覧を見せて")).toHaveLength(1);
  });
});
