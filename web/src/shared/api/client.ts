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

/** A confirmed call to execute, posted to POST /api/invoke. */
export type InvokeRequest = components["schemas"]["InvokeRequest"];

/**
 * The body of a successful POST /api/invoke.
 *
 * Same reasoning as `PlanResult` above: openapi-fetch's own `MethodResponse`
 * rather than `components["schemas"]["InvokeResult"]`, so
 * `exactOptionalPropertyTypes` sees the type the client actually produces.
 */
export type InvokeResult = MethodResponse<typeof client, "post", "/api/invoke">;

/** Calls POST /api/invoke and returns the executed call's rendered result. */
export async function postInvoke(request: InvokeRequest): Promise<InvokeResult> {
  // See the comment on getHealth above: fetch is read at call time on purpose.
  const { data, error } = await client.POST("/api/invoke", {
    body: request,
    fetch: globalThis.fetch,
  });

  if (error !== undefined) {
    throw new Error("POST /api/invoke failed");
  }

  return data;
}

/**
 * The body of a successful GET /api/workspaces: every row the stub owner
 * has, most recently created last.
 *
 * openapi-fetch's own `MethodResponse`, not `components["schemas"]` mapped
 * over an array: for an array response the two are structurally close but
 * not identical types, the same gap `PlanResult` above works around for
 * `exactOptionalPropertyTypes`. This is the shape the client actually
 * produces.
 */
export type WorkspacesList = MethodResponse<typeof client, "get", "/api/workspaces">;

/** One row of `WorkspacesList` - no panel data, only a count. */
export type WorkspaceSummary = WorkspacesList[number];

/** Calls GET /api/workspaces and returns the stub owner's workspaces. */
export async function listWorkspaces(): Promise<WorkspacesList> {
  // See the comment on getHealth above: fetch is read at call time on purpose.
  const { data, error } = await client.GET("/api/workspaces", { fetch: globalThis.fetch });

  if (error !== undefined) {
    throw new Error("GET /api/workspaces failed");
  }

  return data;
}

/** A new, empty workspace to create. */
export type CreateWorkspaceRequest = components["schemas"]["CreateWorkspaceRequest"];

/** The body of a successful POST /api/workspaces. See the comment on PlanResult above. */
export type WorkspaceCreated = MethodResponse<typeof client, "post", "/api/workspaces">;

/** Calls POST /api/workspaces and returns the workspace just created. */
export async function createWorkspace(request: CreateWorkspaceRequest): Promise<WorkspaceCreated> {
  // See the comment on getHealth above: fetch is read at call time on purpose.
  const { data, error } = await client.POST("/api/workspaces", {
    body: request,
    fetch: globalThis.fetch,
  });

  if (error !== undefined) {
    throw new Error("POST /api/workspaces failed");
  }

  return data;
}

/**
 * Calls DELETE /api/workspaces/{id}. Deleting a workspace that does not exist
 * is not an error (docs/specs/workspaces.md section 5): the end state is the
 * same either way, so this only rejects on a transport failure.
 */
export async function deleteWorkspace(id: string): Promise<void> {
  // See the comment on getHealth above: fetch is read at call time on purpose.
  const { error } = await client.DELETE("/api/workspaces/{id}", {
    params: { path: { id } },
    fetch: globalThis.fetch,
  });

  if (error !== undefined) {
    throw new Error("DELETE /api/workspaces/{id} failed");
  }
}
