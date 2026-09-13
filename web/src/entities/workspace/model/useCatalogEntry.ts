import { useEffect, useState } from "react";

import { getCatalog, type CatalogEntry } from "@/shared/api/catalog";

export interface CatalogLookup {
  /**
   * `true` once `GET /api/catalog` has settled, successfully or not. A
   * panel must not decide it is safe to auto-invoke before this is `true`
   * (`usePanelInvoke`'s own `enabled`) - deciding on `false`'s starting
   * value would let a fast `/api/invoke` race a slow catalogue fetch and
   * write a row before the safety check it exists to make ever resolves.
   */
  readonly ready: boolean;
  /**
   * This panel's own entry in the signed-in person's catalogue, matched
   * by `service` and `operationId` - `undefined` once `ready` if the
   * catalogue lists no such operation (a permission revoked since the
   * panel was saved, say). A caller reads `undefined` the same way it
   * would read an operation it knows nothing is wrong with: `/api/invoke`
   * still enforces its own permission check, and a call it refuses
   * surfaces through the ordinary failure path (AC-W-106), not through
   * this hook pretending to know more than the catalogue told it.
   */
  readonly entry: CatalogEntry | undefined;
}

/**
 * Looks up one operation's own entry in `GET /api/catalog` - the only
 * source `PanelResult` trusts to decide whether a panel's operation is
 * unsafe (`docs/specs/dashboard.md` P14, section 6b).
 *
 * A `Panel`'s own `component` cannot answer that question: it is a
 * person's display choice (P11, `docs/specs/dashboard.md`), carried on the
 * saved panel and changed by `PATCH` to any value the `Component` enum
 * allows, with nothing on the server checking it against the operation
 * behind it - so a panel over an unsafe create could carry
 * `component: "table"` and this hook would still be the only thing that
 * knows better. `CatalogEntry.component`, by contrast, is `domain.Render`
 * run fresh against the operation's own contract on every
 * `GET /api/catalog` (`services/platform/internal/usecase/catalog.go`,
 * `toCatalogEntry`) - nothing is stored, so there is nothing to drift: an
 * operation with a request body answers `form` here every time, the same
 * way it would if `/api/plan` were asked to render a call to it that had
 * not been made yet.
 *
 * A fetch failure (network, 401, ...) is read the same as "not found":
 * see `entry`'s own doc comment.
 */
export function useCatalogEntry(service: string, operationId: string): CatalogLookup {
  const [ready, setReady] = useState(false);
  const [entry, setEntry] = useState<CatalogEntry>();

  useEffect(() => {
    let cancelled = false;

    void (async (): Promise<void> => {
      let found: CatalogEntry | undefined;

      try {
        const entries = await getCatalog();

        found = entries.find((e) => e.service === service && e.operationId === operationId);
      } catch {
        found = undefined;
      }

      if (!cancelled) {
        setEntry(found);
        setReady(true);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [service, operationId]);

  return { ready, entry };
}
