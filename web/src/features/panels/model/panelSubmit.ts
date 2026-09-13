import { useState } from "react";

import { type AddPanelRequest, type Component, type View } from "@/shared/api/client";
import type { CatalogEntry } from "@/shared/api/catalog";
import { useSubmission } from "@/shared/lib/useSubmission";

import { buildView, compactArgs } from "./panelViewRequest";
import type { PanelFields } from "./usePanelFields";

/**
 * The panel's own fields, assembled from a `PanelFields`'s current state -
 * the part of a create or edit request every caller (`usePanelBuilder`,
 * `usePanelEditor`, `usePanelProposal`) builds the same way, before its own
 * shape (a full `AddPanelRequest`, a full `UpdatePanelRequest`) is added
 * around it.
 */
export interface PanelPayload {
  readonly args: Record<string, unknown>;
  readonly component: Component;
  readonly title: string;
  readonly view: View | undefined;
}

function buildPayload(entry: CatalogEntry, fields: PanelFields): PanelPayload {
  return {
    args: compactArgs(entry.schema, fields.argsValues),
    component: fields.component,
    title: fields.title.trim(),
    view: buildView(
      fields.component,
      fields.category,
      fields.value,
      fields.kind,
      fields.transformEnabled,
      fields.groupBy,
      fields.aggregate,
      fields.aggregateField,
    ),
  };
}

/** A `PanelPayload` as `POST /api/workspaces/{id}/panels` takes it - shared by `usePanelBuilder` (create) and `usePanelProposal` (place a proposal), the two callers that post rather than patch. */
export function buildAddPanelRequest(entry: CatalogEntry, payload: PanelPayload): AddPanelRequest {
  return {
    service: entry.service,
    operationId: entry.operationId,
    args: payload.args,
    component: payload.component,
    title: payload.title,
    ...(payload.view === undefined ? {} : { view: payload.view }),
  };
}

export interface PanelSave {
  readonly submitting: boolean;
  readonly error: string | null;
  readonly handleSave: () => void;
}

/**
 * The save button's own behaviour, shared by `usePanelBuilder` (create,
 * where `entry` comes from the picker and may still be null),
 * `usePanelEditor` (edit an existing panel) and `usePanelProposal` (place a
 * proposal) - the three hooks behind `AddPanelForm` (`docs/specs/dashboard.md`
 * P12, P7; `docs/specs/proposing.md` section 5). Checks
 * `fields.missingBeforeSave()` first, and only then builds the request's
 * common fields and hands them to `submit` alongside the entry that named
 * them; `onSaved` runs once `submit` resolves, so each caller's own side
 * effects after a save (closing the form, clearing the picker) stay where
 * they were before this was pulled out from under all three -
 * `make guard-duplication` is what asked for that: the three hooks' own
 * save logic was identical past the one call each of them makes.
 */
export function usePanelSave<T>(
  fields: PanelFields,
  entry: CatalogEntry | null,
  submit: (entry: CatalogEntry, payload: PanelPayload) => Promise<T>,
  onSaved: (result: T) => void,
): PanelSave {
  const { submitting, error, run } = useSubmission();
  const [incomplete, setIncomplete] = useState<string | null>(null);

  const handleSave = (): void => {
    if (entry === null) {
      return;
    }

    const missing = fields.missingBeforeSave();

    if (missing !== null) {
      setIncomplete(missing);

      return;
    }

    setIncomplete(null);

    void run(async () => {
      const result = await submit(entry, buildPayload(entry, fields));

      onSaved(result);
    });
  };

  return { submitting, error: error ?? incomplete, handleSave };
}
