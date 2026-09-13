import type { ReactNode } from "react";

import type { Component, PlanResult } from "@/shared/api/client";
import type { ProposedPanel } from "@/shared/api/panels";

/**
 * What a `table`/`detail` result answer needs the save control drawn with -
 * exactly the provenance and widget it is already showing, plus the
 * question that produced it as the title's default.
 *
 * Split out of `TurnList.tsx` alongside `ProposalSlotProps` only so that
 * file stays under eslint's `max-lines` once it also draws a `proposal`
 * answer turn - both slot shapes, and the render-prop types built from
 * them, are shared by `TurnList.tsx` and `renderResultAnswer.tsx`.
 */
export interface SaveControlSlotProps {
  readonly source: NonNullable<PlanResult["source"]>;
  readonly component: Component;
  /** The answer's own `view` (only a chart-hinted result carries one), forwarded so saving it carries the axes too (AC-P-105). */
  readonly view?: PlanResult["view"];
  readonly defaultTitle: string;
}

/** What a `proposal` answer needs its form drawn with - the panel the platform filled in. */
export interface ProposalSlotProps {
  readonly panel: ProposedPanel;
}

/**
 * Draws a result turn's "save to a workspace" control. Left as a slot,
 * rather than `TurnList` importing `SaveToWorkspaceControl` itself, because
 * that control lives in `features/workspaces` and `features/conversation`
 * cannot import a sibling feature (`make guard-fsd`). The page that owns
 * both features (`widgets/conversation`) supplies the slot; undefined
 * draws no control at all, which is what `Conversation`'s own tests render
 * without.
 */
export type SaveControlSlot = (props: SaveControlSlotProps) => ReactNode;

/**
 * Draws a `proposal` answer turn's form - the same reasoning as
 * `SaveControlSlot`: the form comes from `features/panels`, a sibling
 * `features/conversation` cannot import. Left undefined draws no proposal
 * at all, which is exactly what the chat screen's own conversation needs
 * (N4, AC-N-104) - `widgets/conversation`'s `ConversationPanel` is where
 * that decision is made, not here: `TurnList` only ever asks "was I handed
 * something to draw a proposal with".
 */
export type ProposalSlot = (props: ProposalSlotProps) => ReactNode;
