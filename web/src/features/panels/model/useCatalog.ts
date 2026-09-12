import { useState } from "react";

import { getCatalog, type CatalogEntry } from "@/shared/api/catalog";

const LOAD_FAILURE = "カタログの取得に失敗しました。";

export interface Catalog {
  readonly loaded: boolean;
  readonly loadError: string | null;
  readonly entries: readonly CatalogEntry[];
  /** Loads the catalogue once. A second call before the first resolves, or after it succeeded, is a no-op. */
  readonly load: () => void;
}

/**
 * Loads `GET /api/catalog` on demand rather than on every workspace visit -
 * the same lazy-on-first-open shape `useSaveToWorkspace`
 * (`features/workspaces`) uses for `GET /api/workspaces`: a panel is added
 * rarely enough that fetching the whole catalogue up front would be wasted
 * work for the common case of a workspace nobody is editing right now.
 */
export function useCatalog(): Catalog {
  const [loaded, setLoaded] = useState(false);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [entries, setEntries] = useState<readonly CatalogEntry[]>([]);

  const load = (): void => {
    if (loaded || loading) {
      return;
    }

    setLoading(true);

    void (async () => {
      try {
        const data = await getCatalog();

        setEntries(data);
      } catch {
        setLoadError(LOAD_FAILURE);
      } finally {
        setLoading(false);
        setLoaded(true);
      }
    })();
  };

  return { loaded, loadError, entries, load };
}
