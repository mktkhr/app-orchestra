import createClient, { type MethodResponse } from "openapi-fetch";

import type { components, paths } from "./gen/platform";

/**
 * The one HTTP client in the application. Nothing outside src/shared/api may
 * import "openapi-fetch" or call fetch directly (see harness/quality/oxlint).
 */
const client = createClient<paths>({ baseUrl: "" });

/** Notified through {@link onUnauthorized} whenever any call here gets a 401. */
const unauthorizedListeners = new Set<() => void>();

client.use({
  onResponse({ response }) {
    if (response.status === 401) for (const listener of unauthorizedListeners) listener();
  },
});

/**
 * Subscribes to every 401 this client receives, from any endpoint.
 * `features/session` uses this to drop back to the sign-in screen the
 * moment a session stops being valid, rather than waiting for whichever
 * screen is open to notice on its own.
 *
 * @returns a function that unsubscribes.
 */
export function onUnauthorized(listener: () => void): () => void {
  unauthorizedListeners.add(listener);

  return () => {
    unauthorizedListeners.delete(listener);
  };
}

/** The signed-in user, as GET/POST /api/session return it (docs/specs/auth.md section 6). */
export type SessionUser = MethodResponse<typeof client, "get", "/api/session">;
/**
 * Calls GET /api/session. Unlike every other call here, a 401 is the
 * ordinary answer to "is anyone signed in", so this resolves to `null`
 * rather than throwing on one.
 */
export async function getSession(): Promise<SessionUser | null> {
  const { data, response } = await client.GET("/api/session", { fetch: globalThis.fetch });

  if (response.status === 401) {
    return null;
  }

  if (data === undefined) {
    throw new Error("GET /api/session failed");
  }

  return data;
}

/** A name and password to check against the platform's accounts. */
export type SignInRequest = components["schemas"]["SignInRequest"];

/** Calls POST /api/session. On success the platform sets the session cookie and returns the user. */
export async function postSession(request: SignInRequest): Promise<SessionUser> {
  const { data, error } = await client.POST("/api/session", {
    body: request,
    fetch: globalThis.fetch,
  });

  if (error !== undefined) {
    throw new Error("POST /api/session failed");
  }

  return data;
}

/** Calls DELETE /api/session. Signing out when nobody is signed in is not an error. */
export async function deleteSession(): Promise<void> {
  const { error } = await client.DELETE("/api/session", { fetch: globalThis.fetch });

  if (error !== undefined) {
    throw new Error("DELETE /api/session failed");
  }
}

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

/**
 * The widget a result is drawn with - `table`, `detail`, `form` or `choice`.
 * Shared by `/api/plan`'s `PlanResult.component` and `/api/invoke`'s
 * `InvokeResult.component`, and by a saved panel's own `component`
 * (`WorkspacePanel`): the same enum names the same four widgets everywhere
 * it appears.
 */
export type Component = components["schemas"]["Component"];

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
 * openapi-fetch's own raw response shape for `GET /api/workspaces/{id}`. Not
 * exported: its `panels` property types as an index signature
 * (`{ readonly [x: number]: Panel }`) rather than a real array - the same
 * gap `WorkspacesList`'s comment above notes for a top-level array
 * response, here one level deeper. `WorkspaceDetail` below is the shape the
 * client actually hands back, with that array made real again.
 */
type WorkspaceDetailRaw = MethodResponse<typeof client, "get", "/api/workspaces/{id}">;

/** One panel of a `WorkspaceDetail` - a saved call, no answer (W2). */
export type WorkspacePanel = WorkspaceDetailRaw["panels"][number];

/** A workspace with its panels, in position order. */
export interface WorkspaceDetail {
  readonly id: WorkspaceDetailRaw["id"];
  readonly name: WorkspaceDetailRaw["name"];
  readonly panels: readonly WorkspacePanel[];
}

/** Calls GET /api/workspaces/{id} and returns the workspace with its panels. */
export async function getWorkspace(id: string): Promise<WorkspaceDetail> {
  // See the comment on getHealth above: fetch is read at call time on purpose.
  const { data, error } = await client.GET("/api/workspaces/{id}", {
    params: { path: { id } },
    fetch: globalThis.fetch,
  });

  if (error !== undefined) {
    throw new Error("GET /api/workspaces/{id} failed");
  }

  // Array.from into a plain array: see the comment on WorkspacesList above -
  // the same openapi-fetch gap, one property deeper, and the same fix.
  return { ...data, panels: Array.from(data.panels) };
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

/**
 * A call to save as a new panel: everything a plan result's `source` and
 * `component` already carry, plus a title a person can edit
 * (docs/specs/workspaces.md section 3). Nothing here is derived - the
 * caller copies `source.service`/`source.operationId`/`source.args` and the
 * result's own `component` straight through.
 */
export type AddPanelRequest = components["schemas"]["CreatePanelRequest"];

/** The body of a successful POST /api/workspaces/{id}/panels. See the comment on PlanResult above. */
export type PanelCreated = MethodResponse<typeof client, "post", "/api/workspaces/{id}/panels">;

/** Calls POST /api/workspaces/{id}/panels and returns the panel just saved. */
export async function addPanel(
  workspaceId: string,
  request: AddPanelRequest,
): Promise<PanelCreated> {
  // See the comment on getHealth above: fetch is read at call time on purpose.
  const { data, error } = await client.POST("/api/workspaces/{id}/panels", {
    params: { path: { id: workspaceId } },
    body: request,
    fetch: globalThis.fetch,
  });

  if (error !== undefined) {
    throw new Error("POST /api/workspaces/{id}/panels failed");
  }

  return data;
}
