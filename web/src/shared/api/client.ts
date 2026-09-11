import createClient, { type MethodResponse } from "openapi-fetch";

import type { components, paths } from "./gen/platform";

/**
 * The one HTTP client in the application. Nothing outside src/shared/api may
 * import "openapi-fetch" or call fetch directly (see harness/quality/oxlint).
 */
const client = createClient<paths>({ baseUrl: "" });

/** The platform's health status, as reported by GET /api/health. */
export interface Health {
  readonly status: string;
}

/** Calls GET /api/health and returns its body. */
export async function getHealth(): Promise<Health> {
  // Read globalThis.fetch at call time, not at client construction time: the
  // client above is a module-level singleton, and openapi-fetch otherwise
  // captures whatever "fetch" was global when it was built.
  const { data, error } = await client.GET("/api/health", { fetch: globalThis.fetch });

  if (error !== undefined) {
    throw new Error("GET /api/health failed");
  }

  return data;
}

/** A question, and any answers to a previous `kind: "ask"` disambiguation. */
export type PlanRequest = components["schemas"]["PlanRequest"];

/**
 * The body of a successful POST /api/plan.
 *
 * openapi-fetch's own MethodResponse, rather than
 * `components["schemas"]["PlanResult"]`: the two describe the same shape,
 * but openapi-fetch's response mapping and the generated alias are
 * equivalent types and not identical ones, which `exactOptionalPropertyTypes`
 * rejects when the client's result is returned as the alias. Both are read
 * out of the same generated `paths`, so the contract is still the only
 * source; this is the spelling the client actually produces.
 */
export type PlanResult = MethodResponse<typeof client, "post", "/api/plan">;

/** Calls POST /api/plan and returns the planner's decision. */
export async function postPlan(request: PlanRequest): Promise<PlanResult> {
  // See the comment on getHealth above: fetch is read at call time on purpose.
  const { data, error } = await client.POST("/api/plan", {
    body: request,
    fetch: globalThis.fetch,
  });

  if (error !== undefined) {
    throw new Error("POST /api/plan failed");
  }

  return data;
}
