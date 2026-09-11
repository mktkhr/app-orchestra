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
 */
export function PanelCardShell({ title, children, action }: PanelCardShellProps): JSX.Element {
  return (
    <Card variant="outlined">
      <CardHeader title={title} action={action} />
      <CardContent>{children}</CardContent>
    </Card>
  );
}
