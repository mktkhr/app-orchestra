import Alert from "@mui/material/Alert";
import CircularProgress from "@mui/material/CircularProgress";
import Stack from "@mui/material/Stack";
import type { JSX } from "react";

import { Provenance, RenderedResult } from "@/entities/rendering";
import { PanelCardShell, usePanelInvoke } from "@/entities/workspace";
import type { WorkspacePanel } from "@/shared/api/client";

interface PanelResultProps {
  readonly panel: WorkspacePanel;
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
 */
export function PanelResult({ panel }: PanelResultProps): JSX.Element {
  const { loading, error, result } = usePanelInvoke(panel);

  return (
    <PanelCardShell title={panel.title}>
      <Provenance
        source={{ service: panel.service, operationId: panel.operationId, args: panel.args }}
      />
      {loading ? (
        <Stack direction="row" sx={{ justifyContent: "center", py: 2 }}>
          <CircularProgress size={24} aria-label="読み込み中" />
        </Stack>
      ) : null}
      {error === null ? null : <Alert severity="error">{error}</Alert>}
      {result === null ? null : (
        <RenderedResult component={result.component} data={result.data} fields={result.fields} />
      )}
    </PanelCardShell>
  );
}
