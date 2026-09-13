import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import CardHeader from "@mui/material/CardHeader";
import type { JSX, ReactNode } from "react";

interface PanelCardShellProps {
  readonly title: string;
  readonly children: ReactNode;
  /**
   * A control drawn in the card header, alongside the title - the refresh
   * control (`usePanelInvoke.refresh`), for instance. Optional: a shell with
   * nothing to act on need not pass one.
   */
  readonly action?: ReactNode;
}

/**
 * The card every workspace panel is drawn in: its title, and whatever body
 * the caller gives it (`docs/specs/workspaces.md` section 8).
 *
 * Presentational only - it does not call `/api/invoke` or know about
 * `Provenance`/`ResultTable`. Those live in `entities/rendering`, and
 * `entities/workspace` may not import a sibling entity slice
 * (`make guard-fsd`), so the panel's provenance and its rendered answer are
 * composed one layer up, in `pages/workspace/ui/PanelResult.tsx`, which can
 * see both. `PanelResult` builds the refresh control there too, for the
 * same reason, and hands it in through `action`.
 *
 * `height: "100%"` rather than an intrinsic, content-sized height
 * (`docs/plans/layout.md` Task 1): `WorkspaceGrid` draws each panel inside
 * a `react-grid-layout` item that is already the exact pixel box `width`
 * and `height` say it should be, and this card is meant to fill that box
 * rather than leave the rest of it blank or, worse, spill past it.
 *
 * `CardContent` no longer scrolls itself (`docs/specs/dashboard.md` P15,
 * DECISIONS.md 2026-09-13): it used to carry its own `overflow: "auto"`,
 * on top of the scrollbar a tall table already draws around itself
 * (`entities/rendering/ui/ResultTableGrid.tsx`'s `TableContainer`), so a
 * panel with more rows than fit showed two. `CardContent` still
 * `flexGrow`s to fill the height the grid gave the card, and is still a
 * column flexbox - `minHeight: 0` is what lets a flex child (the caller's
 * own scrolling body, in `PanelResult`) shrink below its content's
 * intrinsic height instead of forcing this box to grow past its own,
 * which is what `overflow: "hidden"` here would otherwise clip against.
 * `overflow: "hidden"` itself is deliberate, not a leftover default: it is
 * what keeps this box from re-introducing the outer scrollbar `P15`
 * removes, now that AC-P-112's one scroller lives one level down, in
 * whatever result is actually drawn.
 */
export function PanelCardShell({ title, children, action }: PanelCardShellProps): JSX.Element {
  return (
    <Card variant="outlined" sx={{ height: "100%", display: "flex", flexDirection: "column" }}>
      <CardHeader title={title} action={action} />
      <CardContent
        sx={{
          flexGrow: 1,
          minHeight: 0,
          overflow: "hidden",
          display: "flex",
          flexDirection: "column",
        }}
      >
        {children}
      </CardContent>
    </Card>
  );
}
