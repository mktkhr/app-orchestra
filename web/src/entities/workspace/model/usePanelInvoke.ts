import { useEffect, useState } from "react";

import { postInvoke, type InvokeResult, type WorkspacePanel } from "@/shared/api/client";

const LOAD_FAILURE = "結果の取得に失敗しました。";

interface PanelInvoke {
  readonly loading: boolean;
  readonly error: string | null;
  readonly result: InvokeResult | null;
}

/**
 * Runs one panel's saved call through `POST /api/invoke` on mount - a panel
 * holds a question, not an answer (W2, docs/specs/workspaces.md), so every
 * time a card appears it is asked again.
 *
 * Each `PanelCard` calls this on its own: the effect starts as soon as the
 * card mounts and does not wait for any other card's, so one slow service
 * does not hold up a workspace's other panels (AC-W-102's "opening a
 * workspace calls each panel's operation" - independently, not in series).
 * A failure is reported through `error` rather than thrown, so the card
 * that hit it can say so in place while its siblings keep drawing
 * (AC-W-106).
 */
export function usePanelInvoke(panel: WorkspacePanel): PanelInvoke {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<InvokeResult | null>(null);

  useEffect(() => {
    let cancelled = false;

    const load = async (): Promise<void> => {
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

  return { loading, error, result };
}
