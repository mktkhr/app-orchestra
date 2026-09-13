import Box from "@mui/material/Box";
import { useState, type JSX } from "react";

import { Provenance } from "@/entities/rendering";
import { PanelCardShell, useCatalogEntry, usePanelInvoke } from "@/entities/workspace";
import type { PlanResult, WorkspacePanel } from "@/shared/api/client";

import { PanelActions } from "./PanelActions";
import { PanelInvokedBody } from "./PanelInvokedBody";
import { PanelQuickAddBody } from "./PanelQuickAddBody";

interface PanelResultProps {
  readonly workspaceId: string;
  readonly panel: WorkspacePanel;
  /** Threaded straight through to `PanelActions` - see its own doc comment. */
  readonly onMove?: ((panelId: string, direction: "previous" | "next") => void) | undefined;
  readonly onResize?:
    | ((panelId: string, deltaWidth: number, deltaHeight: number) => void)
    | undefined;
}

/**
 * One workspace panel, fully assembled: `entities/workspace`'s card shell
 * around `entities/rendering`'s provenance and result widgets. This is the
 * one place that composes the two - both are entity slices, and
 * `make guard-fsd` forbids one entity importing a sibling, so `PanelCardShell`
 * stays presentational and `RenderedResult`/`Provenance` stay ignorant of
 * panels; a page, which may import either, is where they meet.
 *
 * The answer is asked for on mount (`usePanelInvoke`) rather than passed
 * in: a panel is a saved call, not a saved result (W2,
 * docs/specs/workspaces.md). A failed call shows its message inside this
 * card instead of throwing, so a workspace with one unreachable panel still
 * draws the rest of them (AC-W-106).
 *
 * The refresh control is an `IconButton`, not a disabled `Button
 * variant="contained"`: `make guard-layout` rejects a disabled contained
 * button outright (it has no edge the layout guard can see), and the
 * control must stay usable-looking anyway - AC-W-103 asks that it show it
 * is working, not that it lock itself. So it never disables; while
 * `refreshing` it swaps its icon for a small `CircularProgress` and its
 * label from "更新" to "更新中", and double-clicks are absorbed by
 * `usePanelInvoke`'s own in-flight guard rather than by disabling the
 * button.
 *
 * `panel` is copied into local state (`current`) rather than read straight
 * off the prop: `EditPanelControl` (P12) returns the panel as it reads
 * after a save, and this card draws that answer immediately - AC-P-110's
 * "edited then reloaded draws as edited" holds for the reload half because
 * the platform itself now has the change; this state is what makes it
 * true without one first, for the tab already open.
 *
 * `useCatalogEntry` decides, before anything else here does, whether
 * `current`'s operation is unsafe (`docs/specs/dashboard.md` P14, section
 * 6b): `usePanelInvoke`'s own `enabled` stays `false` until that lookup
 * resolves, so a safe panel's first `/api/invoke` never races ahead of the
 * check, and stays `false` forever once the lookup says `form` - an unsafe
 * operation is answered by `PanelQuickAddBody`'s form, never by this
 * card's own mount or its refresh control, which for that case only
 * clears the last submission instead of calling anything (AC-P-111).
 *
 * The body - `PanelInvokedBody` or `PanelQuickAddBody` - sits in a `Box`
 * that fills whatever height `PanelCardShell`'s `CardContent` has left
 * (`flexGrow: 1`, `minHeight: 0`) and scrolls on its own only when nothing
 * inside it already does: a table's own `TableContainer` is the one
 * scroller for a table result (`ResultTable`'s own doc comment), so this
 * box stays `overflow: "hidden"` for that case and falls back to
 * `overflow: "auto"` for every other shape a result or a form can take
 * (`docs/specs/dashboard.md` P15, AC-P-112).
 */
export function PanelResult({
  workspaceId,
  panel,
  onMove,
  onResize,
}: PanelResultProps): JSX.Element {
  const [current, setCurrent] = useState(panel);
  const [submitted, setSubmitted] = useState<PlanResult | null>(null);
  const catalog = useCatalogEntry(current.service, current.operationId);
  const unsafeEntry =
    catalog.ready && catalog.entry?.component === "form" ? catalog.entry : undefined;
  // Not just `unsafeEntry === undefined`: before `catalog.ready` flips
  // true, `unsafeEntry` reads `undefined` too - "not yet known to be
  // unsafe" is not the same claim as "known to be safe", and the first
  // render would otherwise enable `usePanelInvoke` for the one instant it
  // takes the catalogue lookup to resolve (`docs/specs/dashboard.md` P14,
  // AC-P-111).
  const { loading, refreshing, error, result, refresh } = usePanelInvoke(
    current,
    catalog.ready && unsafeEntry === undefined,
  );

  const isTableResult =
    unsafeEntry === undefined
      ? result !== null && current.view?.chart === undefined && result.component === "table"
      : submitted !== null && submitted.component === "table";

  return (
    <PanelCardShell
      title={current.title}
      action={
        <PanelActions
          workspaceId={workspaceId}
          panel={current}
          refreshing={unsafeEntry === undefined && refreshing}
          onRefresh={
            unsafeEntry === undefined
              ? refresh
              : () => {
                  setSubmitted(null);
                }
          }
          onSaved={setCurrent}
          onMove={onMove}
          onResize={onResize}
        />
      }
    >
      <Provenance
        source={{
          service: current.service,
          // A saved Panel stores only the identifier (W2,
          // docs/specs/workspaces.md) - it never re-fetches the catalogue
          // just to find the service's display name (P9,
          // docs/specs/dashboard.md), so this reads the same as it always
          // has here: the identifier, unlike the chat/plan provenance
          // above it, which does carry the contract's own name
          // (DECISIONS.md, 2026-09-13).
          serviceDisplayName: current.service,
          operationId: current.operationId,
          args: current.args,
        }}
      />
      <Box
        sx={{
          flexGrow: 1,
          minHeight: 0,
          display: "flex",
          flexDirection: "column",
          overflow: isTableResult ? "hidden" : "auto",
        }}
      >
        {unsafeEntry === undefined ? (
          <PanelInvokedBody
            loading={loading}
            error={error}
            result={result}
            view={current.view}
            title={current.title}
          />
        ) : (
          <PanelQuickAddBody
            entry={unsafeEntry}
            initialArgs={current.args}
            submitted={submitted}
            onSubmitted={setSubmitted}
          />
        )}
      </Box>
    </PanelCardShell>
  );
}
