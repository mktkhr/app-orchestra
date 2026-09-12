import type { MethodResponse } from "openapi-fetch";

import { client } from "./client";

/**
 * `GET /api/catalog` (`docs/specs/dashboard.md` section 5). Split out of
 * `client.ts` only because that file is already at eslint's `max-lines`
 * (300) - same reasoning as `users.ts`'s own header comment - and reuses
 * that file's one `client` instance rather than a `createClient` of its own.
 */

/**
 * The body of a successful GET /api/catalog: every operation the
 * signed-in person may call, each with enough to build a panel over it -
 * its arguments schema, its response fields, and, when the contract
 * declares `x-ui-hint.chart`, that chart's axes as `view`.
 */
export type CatalogList = MethodResponse<typeof client, "get", "/api/catalog">;

/** One row of `CatalogList`. */
export type CatalogEntry = CatalogList[number];

/** Calls GET /api/catalog and returns the signed-in person's own catalogue. */
export async function getCatalog(): Promise<readonly CatalogEntry[]> {
  // fetch is read at call time, not at client construction time - see the
  // comment on getHealth in client.ts.
  const { data, error } = await client.GET("/api/catalog", { fetch: globalThis.fetch });

  if (error !== undefined) {
    throw new Error("GET /api/catalog failed");
  }

  return Array.from(data);
}
