import { App } from "@app-orchestra/web";
import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vite-plus/test";

/**
 * Application-level: renders the real App (AppBar + Drawer shell + chat
 * page) against a scripted backend, the way the built product will
 * actually run.
 */
describe("App", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders the chat navigation and reaches the backend", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() =>
        Promise.resolve(
          new Response(JSON.stringify({ status: "ok" }), {
            status: 200,
            headers: { "content-type": "application/json" },
          }),
        ),
      ),
    );

    render(<App />);

    expect(screen.getAllByText("チャット").length).toBeGreaterThan(0);
    expect(await screen.findByText(/バックエンド応答: ok/u)).toBeTruthy();
  });
});
