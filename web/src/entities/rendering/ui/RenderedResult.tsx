import type { JSX } from "react";

import type { Component } from "@/shared/api/client";

import type { Fields } from "../model/rows";
import { ResultDetail } from "./ResultDetail";
import { ResultTable } from "./ResultTable";

interface RenderedResultProps {
  readonly component: Component;
  readonly data: Record<string, unknown> | undefined;
  readonly fields?: Fields | undefined;
}

/**
 * Draws `data` with the widget `component` names - a table or a detail
 * list - or nothing when there is no `data` to draw. This is the one place
 * that maps a `Component` to the entity that draws it; both a chat answer
 * (`features/conversation/ui/TurnList.tsx`) and a workspace panel
 * (`entities/workspace/ui/PanelCard.tsx`) call it instead of repeating the
 * same `if (component === "table")` / `if (component === "detail")` branch
 * (`harness/quality/duplication.txt`).
 *
 * `form` and `choice` are not handled here: those results are not a saved
 * call's answer, they are a request for one - a workspace panel is a call
 * that already ran (W1, docs/specs/workspaces.md), so a panel never names
 * either.
 */
export function RenderedResult({
  component,
  data,
  fields,
}: RenderedResultProps): JSX.Element | null {
  if (data === undefined) {
    return null;
  }

  if (component === "table") {
    return <ResultTable data={data} fields={fields} />;
  }

  if (component === "detail") {
    return <ResultDetail data={data} fields={fields} />;
  }

  return null;
}
