import { useEffect, useRef, useState } from "react";

import { postInvoke, type InvokeResult, type WorkspacePanel } from "@/shared/api/client";

const LOAD_FAILURE = "結果の取得に失敗しました。";

interface PanelInvoke {
  readonly loading: boolean;
  readonly refreshing: boolean;
  readonly error: string | null;
  readonly result: InvokeResult | null;
  /**
   * Posts the panel's call to `/api/invoke` again and replaces `result` on
   * success (AC-W-103, W2). A no-op while a call - the initial load or an
   * earlier refresh - is already in flight.
   */
  readonly refresh: () => void;
}

/**
 * Runs one panel's saved call through `POST /api/invoke` on mount - a panel
 * holds a question, not an answer (W2, docs/specs/workspaces.md), so every
 * time a card appears it is asked again. `refresh` runs the same call again
 * on demand, for the card's refresh control.
 *
 * Each `PanelCard` calls this on its own: the effect starts as soon as the
 * card mounts and does not wait for any other card's, so one slow service
 * does not hold up a workspace's other panels (AC-W-102's "opening a
 * workspace calls each panel's operation" - independently, not in series).
 * A failure is reported through `error` rather than thrown, so the card
 * that hit it can say so in place while its siblings keep drawing
 * (AC-W-106).
 *
 * A refresh that fails leaves `result` as it was: a stale table is still a
 * table a person can read, and this is a dashboard, not a form - replacing
 * it with nothing on a transient failure would throw away a correct answer
 * over a network blip. The error is still reported alongside it, so a
 * refresh that keeps failing does not read as a refresh that keeps
 * succeeding.
 *
 * `inFlightRef` is checked and set synchronously inside `refresh`, before
 * any `setState` call: `loading`/`refreshing` only take effect on the next
 * render, so a second click arriving before that render would otherwise
 * read the stale `false` and fire a second request. The ref has no such
 * delay.
 */
export function usePanelInvoke(panel: WorkspacePanel): PanelInvoke {
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<InvokeResult | null>(null);
  const inFlightRef = useRef(false);

  useEffect(() => {
    let cancelled = false;

    const load = async (): Promise<void> => {
      inFlightRef.current = true;
      setLoading(true);
      setError(null);
      setResult(null);

      try {
        const data = await postInvoke({
          service: panel.service,
          operationId: panel.operationId,
          args: panel.args,
        });

        if (!cancelled) {
          setResult(data);
        }
      } catch {
        if (!cancelled) {
          setError(LOAD_FAILURE);
        }
      } finally {
        inFlightRef.current = false;
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    void load();

    return () => {
      cancelled = true;
    };
  }, [panel]);

  const refresh = (): void => {
    if (inFlightRef.current) {
      return;
    }
    inFlightRef.current = true;
    setRefreshing(true);
    setError(null);

    void (async () => {
      try {
        const data = await postInvoke({
          service: panel.service,
          operationId: panel.operationId,
          args: panel.args,
        });
        setResult(data);
      } catch {
        setError(LOAD_FAILURE);
      } finally {
        inFlightRef.current = false;
        setRefreshing(false);
      }
    })();
  };

  return { loading, refreshing, error, result, refresh };
}
