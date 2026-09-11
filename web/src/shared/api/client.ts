import createClient from "openapi-fetch";

import type { paths } from "./gen/platform";

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
