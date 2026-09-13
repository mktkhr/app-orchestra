import type { MethodResponse } from "openapi-fetch";

import { client } from "./client";
import type { components } from "./gen/platform";

/**
 * `PATCH /api/workspaces/{id}/panels/{panelId}` (`docs/specs/dashboard.md`
 * section 6a). Split out of `client.ts` only because that file is already
 * at eslint's `max-lines` (300) - same reasoning as `catalog.ts`'s own
 * header comment - and reuses that file's one `client` instance rather
 * than a `createClient` of its own.
 */

/**
 * Fields to change on an existing panel (P11, section 6a). Every field is
 * optional - only the ones a caller sets go out on the wire, and only
 * those change (AC-P-108). `view` carries a third state a plain optional
 * field cannot: passing `null` removes the panel's view, and leaving the
 * key out of the object entirely leaves the view as it was - the same
 * "absent vs null" distinction the generated type keeps, straight from the
 * contract's `nullable: true` (see `openapi.yaml`'s
 * `UpdatePanelRequest.view`). `JSON.stringify` already drops a key whose
 * value is `undefined`, so building this object with spreads and never
 * assigning a field a caller did not name is enough to get "absent" for
 * free; assigning `null` explicitly is how a caller asks for "removed".
 */
export type UpdatePanelRequest = components["schemas"]["UpdatePanelRequest"];

/**
 * The panel a `propose_panel` call filled in (`docs/specs/proposing.md`
 * section 3-4), carried on a `PlanResult` whose `kind` is `"proposal"`.
 * Everything a saved `WorkspacePanel` already has, minus `position`,
 * `width` and `height` - a proposal is not placed yet, so it has no
 * position to carry.
 */
export type ProposedPanel = components["schemas"]["ProposedPanel"];

/** The body of a successful PATCH /api/workspaces/{id}/panels/{panelId}. */
export type PanelUpdated = MethodResponse<
  typeof client,
  "patch",
  "/api/workspaces/{id}/panels/{panelId}"
>;

/**
 * Calls PATCH /api/workspaces/{id}/panels/{panelId} and returns the panel
 * as it reads after the change (AC-P-108, AC-P-110).
 */
export async function patchPanel(
  workspaceId: string,
  panelId: string,
  request: UpdatePanelRequest,
): Promise<PanelUpdated> {
  // fetch is read at call time, not at client construction time - see the
  // comment on getHealth in client.ts.
  const { data, error } = await client.PATCH("/api/workspaces/{id}/panels/{panelId}", {
    params: { path: { id: workspaceId, panelId } },
    body: request,
    fetch: globalThis.fetch,
  });

  if (error !== undefined) {
    throw new Error("PATCH /api/workspaces/{id}/panels/{panelId} failed");
  }

  return data;
}
