import type { JSX } from "react";

import { Conversation, type ProposalSlotProps } from "@/features/conversation";
import { ProposalControl } from "@/features/panels";
import { SaveToWorkspaceControl } from "@/features/workspaces";
import type { WorkspacePanel } from "@/shared/api/client";

interface ConversationPanelProps {
  /**
   * The workspace this conversation is asked from, if any. Passed straight
   * through to `SaveToWorkspaceControl` as `defaultWorkspaceId`, so a
   * result's save control defaults to the workspace it was asked from
   * (`docs/plans/workspaces.md` Task 6). Omitted on the chat screen, which
   * sits on no particular workspace.
   *
   * Also **the decision that only a workspace's own conversation offers a
   * proposal** (`docs/specs/proposing.md` N4, AC-N-104): whether
   * `Conversation` is given a `renderProposal` slot at all turns on
   * whether this is defined, below. `TurnList`/`Conversation`
   * (`features/conversation`) only ever ask "was I handed something to
   * draw a proposal with" - they carry no notion of a workspace at all, so
   * they cannot be the thing that decides a proposal needs one. This
   * widget already is the one place that knows both things: which screen
   * is asking (`pages/chat` calls it with no id, `pages/workspace` always
   * with one) and how to place a panel (`features/panels`) - the same
   * reason `renderSaveControl` already lives here rather than in
   * `TurnList` itself.
   */
  readonly defaultWorkspaceId?: string | undefined;
  /**
   * Called with the panel `POST /api/workspaces/{id}/panels` just
   * returned once a proposal is placed, so `pages/workspace` can draw it
   * in the grid immediately - the same `onAdded` shape `AddPanelControl`
   * already offers. Unused (and the proposal slot never offered) when
   * `defaultWorkspaceId` is undefined.
   */
  readonly onPanelPlaced?: ((panel: WorkspacePanel) => void) | undefined;
}

/**
 * The conversation feature plus the workspaces feature's save control,
 * composed once. Both `pages/chat` and `pages/workspace` need the same
 * conversation wired to the same save control - only whether the control
 * defaults to a particular workspace differs between them - and pages
 * cannot import a sibling page to share that wiring, so it lives here
 * instead (`harness/quality/duplication.txt`, `make guard-duplication`).
 *
 * A widget, not a feature: it composes two, now three, features
 * (`features/conversation`, `features/workspaces`, `features/panels`),
 * which `harness/quality/architecture.json` reserves for the layer above
 * features.
 */
export function ConversationPanel({
  defaultWorkspaceId,
  onPanelPlaced,
}: ConversationPanelProps): JSX.Element {
  return (
    <Conversation
      conversationKey={defaultWorkspaceId ?? "chat"}
      workspaceId={defaultWorkspaceId}
      renderSaveControl={(props) => (
        <SaveToWorkspaceControl {...props} defaultWorkspaceId={defaultWorkspaceId} />
      )}
      renderProposal={
        defaultWorkspaceId === undefined
          ? undefined
          : (props: ProposalSlotProps) => (
              <ProposalControl
                workspaceId={defaultWorkspaceId}
                panel={props.panel}
                onPlaced={onPanelPlaced ?? (() => {})}
              />
            )
      }
    />
  );
}
