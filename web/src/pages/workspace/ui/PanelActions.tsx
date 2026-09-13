import OpenWithIcon from "@mui/icons-material/OpenWith";
import RefreshIcon from "@mui/icons-material/Refresh";
import CircularProgress from "@mui/material/CircularProgress";
import IconButton from "@mui/material/IconButton";
import Stack from "@mui/material/Stack";
import Tooltip from "@mui/material/Tooltip";
import type { JSX, KeyboardEvent } from "react";

import { EditPanelControl } from "@/features/panels";
import type { WorkspacePanel } from "@/shared/api/client";

const ARRANGE_HINT = "矢印キーで並べ替え、Shiftキーを押しながらでサイズ変更します。";

interface PanelActionsProps {
  readonly workspaceId: string;
  readonly panel: WorkspacePanel;
  readonly refreshing: boolean;
  readonly onRefresh: () => void;
  /** Called with the panel a save just returned, so `PanelResult` can draw it immediately (AC-P-110). */
  readonly onSaved: (panel: WorkspacePanel) => void;
  /**
   * The keyboard half of arranging a workspace (`docs/specs/layout.md`
   * AC-L-103), wired in by `WorkspaceGrid` on both breakpoints: a stack of
   * one column still has an order worth moving a panel within, and a
   * height worth changing (section 5a) - only pointer dragging and
   * resizing-by-handle are the narrow breakpoint's own "nothing to do"
   * (section 5). Left `undefined`, this control draws nothing at all; both
   * props are optional together for exactly that case, not because either
   * breakpoint omits one on its own.
   */
  readonly onMove?: ((panelId: string, direction: "previous" | "next") => void) | undefined;
  /**
   * On the wide breakpoint, both the panel's `width` and `height`,
   * `deltaWidth`/`deltaHeight` apart. On the narrow one, `WorkspaceGrid`
   * wires this to a handler that only ever reads `deltaHeight` and changes
   * `narrowHeight` - there is no width to change on one column (L7), so a
   * Shift+Left/Right on a phone is a no-op there rather than reaching for a
   * field that does not exist.
   */
  readonly onResize?:
    | ((panelId: string, deltaWidth: number, deltaHeight: number) => void)
    | undefined;
}

/**
 * `PanelResult`'s own header controls: refresh (`usePanelInvoke.refresh`),
 * edit (`EditPanelControl`, P12) and, when arranging is live, the keyboard
 * arrange control - side by side, in the header `PanelCardShell` already
 * gives every panel, rather than a floating control drawn over the card.
 * An earlier version drew `ArrangeControl` as an overlay at the card's own
 * top-right corner, which is exactly where this header's own buttons sit -
 * `make guard-browser`'s own `dashboard.spec.ts` caught it: the overlay
 * intercepted pointer events meant for "編集" underneath it. Living in the
 * header instead means one row of buttons, none of them fighting another
 * for the same pixels.
 *
 * Split out of `PanelResult` only to keep that file's own import count
 * under `import/max-dependencies` (`eslint`), the same reason its own
 * header comment already gives for reading `600px` directly instead of
 * through `useTheme`.
 */
export function PanelActions({
  workspaceId,
  panel,
  refreshing,
  onRefresh,
  onSaved,
  onMove,
  onResize,
}: PanelActionsProps): JSX.Element {
  const handleArrangeKeyDown = (event: KeyboardEvent<HTMLButtonElement>): void => {
    if (onMove === undefined || onResize === undefined) {
      return;
    }

    switch (event.key) {
      case "ArrowLeft":
      case "ArrowUp":
        event.preventDefault();
        if (event.shiftKey) {
          onResize(panel.id, event.key === "ArrowLeft" ? -1 : 0, event.key === "ArrowUp" ? -1 : 0);
        } else {
          onMove(panel.id, "previous");
        }
        break;
      case "ArrowRight":
      case "ArrowDown":
        event.preventDefault();
        if (event.shiftKey) {
          onResize(panel.id, event.key === "ArrowRight" ? 1 : 0, event.key === "ArrowDown" ? 1 : 0);
        } else {
          onMove(panel.id, "next");
        }
        break;
      default:
        break;
    }
  };

  return (
    <Stack direction="row">
      {onMove === undefined || onResize === undefined ? null : (
        <Tooltip title={ARRANGE_HINT}>
          <IconButton
            aria-label={`${panel.title}をキーボードで並べ替え・サイズ変更`}
            onKeyDown={handleArrangeKeyDown}
          >
            <OpenWithIcon />
          </IconButton>
        </Tooltip>
      )}
      <IconButton onClick={onRefresh} aria-label={refreshing ? "更新中" : "更新"}>
        {refreshing ? <CircularProgress size={20} /> : <RefreshIcon />}
      </IconButton>
      <EditPanelControl workspaceId={workspaceId} panel={panel} onSaved={onSaved} />
    </Stack>
  );
}
