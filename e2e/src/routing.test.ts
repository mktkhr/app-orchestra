import { mkdtempSync, readdirSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterAll, describe, expect, it } from "vite-plus/test";

import { signIn, withSession } from "./helpers/auth";
import { freePort, startBinary, stop, waitForReady, type RunningService } from "./helpers/process";

/**
 * Process-level end to end suite for `docs/plans/routing.md` Task 2, Step
 * 3 (`docs/specs/routing.md` section 7, AC-R-102/AC-R-103) - the half of
 * the spec a unit test cannot prove, because
 * `internal/infra/httpserver/router_test.go` builds a router directly
 * rather than a binary answering real TCP: an unknown path answers the
 * built frontend's own index, a missing asset answers 404 and not HTML,
 * and no `/api/...` path - including one the API does not have - is ever
 * shadowed by that fallback.
 *
 * Modelled on `dashboard.test.ts` and `workspaces.test.ts`: the built
 * platform binary (`make build`) on a free port, serving the real
 * `web/dist` (`ORCHESTRA_STATIC_DIR`, the same variable
 * `e2e/playwright.config.ts` sets for the browser suite) rather than a
 * `t.TempDir()` fixture, so this suite also stands as one more proof that
 * `make build`'s own output is the thing under test.
 */

let platform: RunningService | undefined;

afterAll(async () => {
  await stop(platform);
});

/** The first built `.js` asset under dir, or throws - `make build` always leaves one. */
function requireBuiltAsset(dir: string): string {
  const asset = readdirSync(dir).find((name) => name.endsWith(".js"));

  if (asset === undefined) {
    throw new Error(`${dir} carried no built .js asset to assert against`);
  }

  return asset;
}

describe("an address that is not an API route (AC-R-102, AC-R-103)", () => {
  it("answers the built frontend's index for an unknown path, 404 for a missing asset, and never shadows the API", async () => {
    const platformPort = await freePort();
    const dbPath = join(mkdtempSync(join(tmpdir(), "orchestra-e2e-routing-")), "routing.db");

    platform = {
      process: startBinary("../services/platform/bin/api", {
        ORCHESTRA_PORT: String(platformPort),
        ORCHESTRA_STATIC_DIR: "../web/dist",
        ORCHESTRA_DB_PATH: dbPath,
        ORCHESTRA_ADMIN_PASSWORD: "e2e-admin-password",
        ORCHESTRA_SECURE_COOKIE: "false",
      }),
      port: platformPort,
    };

    const baseUrl = `http://127.0.0.1:${platformPort}`;

    await waitForReady(`${baseUrl}/api/health`, 10_000);

    const indexHtml = await (await fetch(`${baseUrl}/`)).text();

    expect(indexHtml).toContain('<div id="root">');

    // An address `react-router` owns, not a file `web/dist` has: the
    // fallback draws it as the application, the same as "/" itself, which
    // is what lets a reload on a deep link survive (AC-R-101's other
    // half, proven in the browser).
    const unknownPathResponse = await fetch(`${baseUrl}/workspaces/does-not-exist`);

    expect(unknownPathResponse.status).toBe(200);
    expect(unknownPathResponse.headers.get("content-type")).toContain("text/html");
    expect(await unknownPathResponse.text()).toBe(indexHtml);

    // A real built asset - named by `make build`'s own hashed output,
    // never hand-typed - is still itself.
    const assetName = requireBuiltAsset("../web/dist/assets");
    const assetResponse = await fetch(`${baseUrl}/assets/${assetName}`);

    expect(assetResponse.status).toBe(200);
    expect(assetResponse.headers.get("content-type")).toContain("javascript");

    // A path with a file extension but no matching file: a browser asking
    // for a build asset that used to exist (a stale cache after a
    // deploy), or a typo. Answering with `index.html` here would hand a
    // script tag a document it cannot parse (AC-R-102).
    const missingAssetResponse = await fetch(`${baseUrl}/assets/missing-abc123.js`);

    expect(missingAssetResponse.status).toBe(404);
    expect(missingAssetResponse.headers.get("content-type")).not.toContain("text/html");

    // AC-R-103: `/api/...` is untouched by the fallback, including a
    // route the embedded OpenAPI spec does not declare at all - which
    // must still answer as the API (404, from the API's own routing, not
    // the fallback's 200) rather than as the application.
    const session = await signIn(baseUrl, "admin", "e2e-admin-password");
    const unknownApiResponse = await fetch(`${baseUrl}/api/does-not-exist`, withSession(session));

    expect(unknownApiResponse.status).toBe(404);
    expect(unknownApiResponse.headers.get("content-type")).not.toContain("text/html");

    // A real API route keeps answering as itself too - the fallback never
    // reached it in the first place, since NewRouter registers "/api/" as
    // its own mux entry ahead of "/".
    const healthResponse = await fetch(`${baseUrl}/api/health`);

    expect(healthResponse.status).toBe(200);
    expect(await healthResponse.json()).toEqual({ status: "ok" });
  }, 30_000);
});
