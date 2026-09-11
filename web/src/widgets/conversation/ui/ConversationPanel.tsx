import type { JSX } from "react";

import { Conversation } from "@/features/conversation";
import { SaveToWorkspaceControl } from "@/features/workspaces";

interface ConversationPanelProps {
  /**
   * The workspace this conversation is asked from, if any. Passed straight
   * through to `SaveToWorkspaceControl` as `defaultWorkspaceId`, so a
   * result's save control defaults to the workspace it was asked from
   * (`docs/plans/workspaces.md` Task 6). Omitted on the chat screen, which
   * sits on no particular workspace.
   */
  readonly defaultWorkspaceId?: string | undefined;
}

/**
 * The conversation feature plus the workspaces feature's save control,
 * composed once. Both `pages/chat` and `pages/workspace` need the same
 * conversation wired to the same save control - only whether the control
 * defaults to a particular workspace differs between them - and pages
 * cannot import a sibling page to share that wiring, so it lives here
 * instead (`harness/quality/duplication.txt`, `make guard-duplication`).
 *
 * A widget, not a feature: it composes two features
 * (`features/conversation`, `features/workspaces`), which
 * `harness/quality/architecture.json` reserves for the layer above
 * features.
 */
export function ConversationPanel({ defaultWorkspaceId }: ConversationPanelProps): JSX.Element {
  return (
    <Conversation
      renderSaveControl={(props) => (
        <SaveToWorkspaceControl {...props} defaultWorkspaceId={defaultWorkspaceId} />
      )}
    />
  );
}
