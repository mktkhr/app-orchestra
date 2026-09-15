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
  // draw as their own assistant turn - 「違いましたか？」 - after the
  // result, not as a row inside it, and choosing one re-asks the same
  // question with that alternative's operationId as `preferred`. The
  // user's own words: clicking two different chips one after another used
  // to stack two operations under the same question, because the chips
  // lived inside the result itself; this is the flow that replaced it.
  it("offers alternatives as their own turn after a result, and re-plans with the one chosen", async () => {
    const user = userEvent.setup();

    vi.stubGlobal("fetch", vi.fn(respond));

    render(<App />);

    const example = await screen.findByRole("button", { name: "在庫の一覧を見せて" });
    await user.click(example);
    await screen.findByText("itm-001");

    // 「違いましたか？」 is its own turn, not a row under the table: it
    // still has to appear, and it has to have an unanswered chip to click.
    expect(await screen.findByText("違いましたか？")).toBeTruthy();
    const chip = screen.getByRole("button", { name: "在庫を登録" });
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
    // The label now appears twice: once on the chip, once as the bubble.
    expect(await screen.findAllByText("在庫を登録")).toHaveLength(2);
    expect(screen.getAllByText("在庫の一覧を見せて")).toHaveLength(1);

    // The chosen chip stays visible and reads as selected; it - and any
    // other chip on the same turn - is no longer a clickable button, so a
    // second click can no longer stack a second operation under the same
    // 「違いましたか？」 (the user's own complaint this flow fixes).
    expect(screen.queryByRole("button", { name: "在庫を登録" })).toBeNull();
    const chosenChips = Array.from(document.querySelectorAll(".MuiChip-colorPrimary"));
    expect(chosenChips.map((el) => el.textContent)).toEqual(["在庫を登録"]);
  });
});
