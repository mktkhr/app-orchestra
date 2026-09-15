import CircularProgress from "@mui/material/CircularProgress";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import type { JSX } from "react";

import type { PlanResult } from "@/shared/api/client";

import type { Turn } from "../model/turn";
import { BUBBLE_SX } from "./bubbleStyles";
import { nearestQuestion } from "./renderResultAnswer";
import { TurnItem } from "./TurnItem";
import type { ProposalSlot, SaveControlSlot } from "./TurnItem";

export type { ProposalSlotProps, SaveControlSlotProps } from "./TurnItem";

interface TurnListProps {
  readonly turns: readonly Turn[];
  /**
   * Whether the most recent question is still waiting on its answer
   * (`Conversation`'s own `pending`, from `useConversation`). The store adds
   * the question turn before the request resolves, so by the time this is
   * true it is already the last turn in `turns`; this draws the spinner
   * that follows it (C4, AC-C-104) and nothing when it isn't.
   */
  readonly pending: boolean;
  /**
   * Forwarded down to every `ResultForm`/`ResultChoice` this list renders.
   * Defined in `Conversation` (the state owner) and passed down rather than
   * imported, because both live in `entities/rendering` and cannot reach up
   * into `features/conversation` for the `Turn` type or `setTurns` itself.
   */
  readonly onFormSubmitted: (result: PlanResult) => void;
  /**
   * Chosen when a person clicks one of an `alternatives` turn's
   * `AlternativesRow` chips - that turn's own id (so the store can mark it
   * answered), the question that produced it (see `nearestQuestion`
   * below), the alternative's own `operationId` as `preferred`, and its
   * `displayName` as `label` (docs/specs/shortlisting.md, section 4).
   * `Conversation` re-asks with `preferred` and `label`, the same way
   * `QuestionForm`'s own submit sends a plain question - but the store
   * appends a choice turn, not a second question turn, for this one, and
   * marks the `alternatives` turn `chosen` before that request even
   * resolves.
   */
  readonly onAlternativeChosen: (
    turnId: string,
    question: string,
    preferred: string,
    label: string,
  ) => void;
  readonly renderSaveControl?: SaveControlSlot | undefined;
  readonly renderProposal?: ProposalSlot | undefined;
}

/** The conversation's turns, in order: a question, then the answer to it. */
export function TurnList({
  turns,
  pending,
  onFormSubmitted,
  onAlternativeChosen,
  renderSaveControl,
  renderProposal,
}: TurnListProps): JSX.Element {
  return (
    <Stack spacing={2}>
      {turns.map((turn, index) => (
        <TurnItem
          key={turn.id}
          turn={turn}
          nearestQuestion={nearestQuestion(turns, index)}
          onFormSubmitted={onFormSubmitted}
          onAlternativeChosen={onAlternativeChosen}
          renderSaveControl={renderSaveControl}
          renderProposal={renderProposal}
        />
      ))}
      {pending ? <PendingAnswer /> : null}
    </Stack>
  );
}

/**
 * The answer's position, while there is none yet (C4). A spinner and one
 * honest line - not a percentage or a step name, because a single LLM call
 * has no intermediate state to report (C5) - gone as soon as the answer
 * turn is drawn or the request fails (AC-C-105), since `pending` alone
 * decides whether this renders at all.
 */
function PendingAnswer(): JSX.Element {
  return (
    <Paper
      elevation={1}
      sx={{
        ...BUBBLE_SX,
        alignSelf: "flex-start",
        p: 2,
        display: "flex",
        alignItems: "center",
        gap: 1.5,
      }}
    >
      <CircularProgress size={20} aria-label="回答を生成中" />
      <Typography variant="body1">回答を作成しています…</Typography>
    </Paper>
  );
}
