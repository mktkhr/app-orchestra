import type { MethodResponse } from "openapi-fetch";

import { client } from "./client";
import type { components } from "./gen/platform";

/**
 * The admin's three endpoints (docs/plans/auth.md Task 5, docs/specs/auth.md
 * section 6): reading every account, reading and replacing one account's
 * permissions, and reading the whole catalogue the permission grid groups by
 * service. Split out of `client.ts` only because that file is already at
 * eslint's `max-lines` (300) - this still reuses its one exported `client`,
 * not a `createClient` call of its own (see that file's own doc comment).
 */

/**
 * The body of a successful GET /api/users: every account the platform
 * knows, admin only (docs/specs/auth.md, section 6).
 */
export type UsersList = MethodResponse<typeof client, "get", "/api/users">;

/** One row of `UsersList`. */
export type UserAccount = UsersList[number];

/** Calls GET /api/users and returns every account. Admin only - a non-admin gets 403. */
export async function listUsers(): Promise<readonly UserAccount[]> {
  // fetch is read at call time, not at client construction time - see the
  // comment on getHealth in client.ts.
  const { data, error } = await client.GET("/api/users", { fetch: globalThis.fetch });

  if (error !== undefined) {
    throw new Error("GET /api/users failed");
  }

  // Array.from into a plain array: see the comment on WorkspacesList in
  // client.ts - the same openapi-fetch gap, and the same fix.
  return Array.from(data);
}

/**
 * One operation a person may call (docs/specs/auth.md, section 4, A3) - a
 * service and an operation id, with nothing else. Shared by
 * `Operation.service`/`operationId` and `SetUserPermissionsRequest`'s own
 * `permissions`.
 */
export type Permission = components["schemas"]["Permission"];

/** Calls GET /api/users/{id}/permissions and returns what the account holds. Admin only. */
export async function getUserPermissions(id: string): Promise<readonly Permission[]> {
  const { data, error } = await client.GET("/api/users/{id}/permissions", {
    params: { path: { id } },
    fetch: globalThis.fetch,
  });

  if (error !== undefined) {
    throw new Error("GET /api/users/{id}/permissions failed");
  }

  return Array.from(data);
}

/**
 * Calls PUT /api/users/{id}/permissions: replaces the named account's
 * permissions wholesale with permissions (docs/specs/auth.md, section 6).
 * Admin only.
 */
export async function setUserPermissions(
  id: string,
  permissions: readonly Permission[],
): Promise<void> {
  const { error } = await client.PUT("/api/users/{id}/permissions", {
    params: { path: { id } },
    body: { permissions: Array.from(permissions) },
    fetch: globalThis.fetch,
  });

  if (error !== undefined) {
    throw new Error("PUT /api/users/{id}/permissions failed");
  }
}

/**
 * The body of a successful GET /api/operations: every operation the
 * catalogue holds, across every configured service, admin only
 * (docs/specs/auth.md, section 4). `GET /api/users/{id}/permissions`
 * answers only what one account already holds - this is what the
 * permission grid groups by service to draw the rest of the picture.
 */
export type OperationsList = MethodResponse<typeof client, "get", "/api/operations">;

/** One row of `OperationsList`. */
export type Operation = OperationsList[number];

/** Calls GET /api/operations and returns every operation in the catalogue. Admin only. */
export async function listOperations(): Promise<readonly Operation[]> {
  const { data, error } = await client.GET("/api/operations", { fetch: globalThis.fetch });

  if (error !== undefined) {
    throw new Error("GET /api/operations failed");
  }

  return Array.from(data);
}
