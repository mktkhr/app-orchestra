import type { JSX } from "react";

import { RenderedResult, ResultForm } from "@/entities/rendering";
import type { CatalogEntry } from "@/shared/api/catalog";
import type { PlanResult } from "@/shared/api/client";

interface PanelQuickAddBodyProps {
  readonly entry: CatalogEntry;
  /** Seeds the form from the panel's own saved arguments (W1: a panel is a saved call). */
  readonly initialArgs: Record<string, unknown>;
  /** What the last submission answered, or `null` before the first one this mount. */
  readonly submitted: PlanResult | null;
  readonly onSubmitted: (result: PlanResult) => void;
}

/**
 * A panel over an unsafe operation's own body (`docs/specs/dashboard.md`
 * P14, section 6b, AC-P-111): the same form `ResultForm` already draws for
 * a `kind: "form"` chat answer, seeded from the panel's saved arguments,
 * and never invoked until a person presses its own submit button - opening
 * the workspace and refreshing the panel (`PanelResult`'s own `onRefresh`
 * for this case just clears `submitted`, back to a blank form) call
 * nothing.
 *
 * Once a submission answers, that answer is drawn in its place with the
 * same `RenderedResult` a safe panel's own result uses - a created row is
 * still a row a person may want to see, not a message that vanishes.
 * `PanelResult` holds `submitted` in local state, not `usePanelInvoke`'s
 * own `result`: this call never went through that hook, which stayed
 * disabled for exactly this operation (`docs/specs/dashboard.md` P14).
 */
export function PanelQuickAddBody({
  entry,
  initialArgs,
  submitted,
  onSubmitted,
}: PanelQuickAddBodyProps): JSX.Element {
  if (submitted !== null) {
    return (
      <RenderedResult
        component={submitted.component ?? entry.component}
        data={submitted.data}
        fields={submitted.fields}
      />
    );
  }

  return (
    <ResultForm
      schema={entry.schema}
      initial={initialArgs}
      target={{
        service: entry.service,
        operationId: entry.operationId,
        serviceDisplayName: entry.serviceDisplayName,
      }}
      onSubmitted={onSubmitted}
    />
  );
}
