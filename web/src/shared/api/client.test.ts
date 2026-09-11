import { afterEach, describe, expect, it, vi } from "vite-plus/test";

import { getHealth } from "./client";

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

describe("getHealth", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns the body on success", async () => {
    stubFetch(200, { status: "ok" });

    await expect(getHealth()).resolves.toEqual({ status: "ok" });
  });

  it("throws when the request fails", async () => {
    stubFetch(500, { message: "boom" });

    await expect(getHealth()).rejects.toThrow("GET /api/health failed");
  });
});
