import { afterEach, describe, expect, it, vi } from "vite-plus/test";

import { patchPanel } from "./panels";

function stubFetch(status: number, body: unknown): void {
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(() =>
      Promise.resolve(
        new Response(JSON.stringify(body), {
          status,
          headers: { "content-type": "application/json" },
        }),
      ),
    ),
  );
}

describe("patchPanel", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns the panel as it reads after the change", async () => {
    stubFetch(200, {
      id: "pnl-1",
      workspaceId: "ws-1",
      service: "inventory",
      operationId: "ListInventoryItems",
      args: {},
      component: "table",
      title: "新しいタイトル",
      position: 0,
    });

    await expect(patchPanel("ws-1", "pnl-1", { title: "新しいタイトル" })).resolves.toMatchObject({
      title: "新しいタイトル",
    });
  });

  const SUCCESS_BODY = {
    id: "pnl-1",
    workspaceId: "ws-1",
    service: "inventory",
    operationId: "ListInventoryItems",
    args: {},
    component: "table",
    title: "t",
    position: 0,
  };

  /**
   * Stubs `fetch` to capture the `Request` it was called with -
   * `openapi-fetch`'s own `coreFetch` always calls it with one, never a
   * bare `(url, init)` pair - and returns `SUCCESS_BODY`. `requests` fills
   * in as each call is made, so a test can inspect the one `patchPanel`
   * just sent after awaiting it.
   */
  function stubFetchCapturingRequests(): readonly Request[] {
    const requests: Request[] = [];

    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>((input) => {
        requests.push(input instanceof Request ? input : new Request(input));

        return Promise.resolve(
          new Response(JSON.stringify(SUCCESS_BODY), {
            status: 200,
            headers: { "content-type": "application/json" },
          }),
        );
      }),
    );

    return requests;
  }

  it("sends an explicit null to remove the view, distinct from leaving the key out (section 6a)", async () => {
    const requests = stubFetchCapturingRequests();

    await patchPanel("ws-1", "pnl-1", { view: null });

    await expect(requests[0]?.clone().json()).resolves.toEqual({ view: null });
  });

  it("leaves the view key out of the body entirely when it is not named", async () => {
    const requests = stubFetchCapturingRequests();

    await patchPanel("ws-1", "pnl-1", { title: "t" });

    const sent: unknown = await requests[0]?.clone().json();

    expect(sent).toEqual({ title: "t" });
    expect(sent).not.toHaveProperty("view");
  });

  it("throws when the request fails", async () => {
    stubFetch(404, { message: "not found" });

    await expect(patchPanel("ws-1", "pnl-1", { title: "x" })).rejects.toThrow(
      "PATCH /api/workspaces/{id}/panels/{panelId} failed",
    );
  });
});
