import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";
import type { JSX } from "react";

import type { PlanResult } from "@/shared/api/client";

import type { Turn } from "../model/turn";
import { AlternativesRow } from "./AlternativesRow";
import { AnswerResult } from "./AnswerResult";
import type { ProposalSlot, SaveControlSlot } from "./AnswerResult";
import { BUBBLE_SX } from "./bubbleStyles";

// Re-exported so `TurnList.tsx` can pull both slot types from this module
// instead of also reaching into "./AnswerResult" directly - one fewer
// distinct dependency there, the same reasoning `AnswerResult.tsx` itself
// gives for re-exporting from "./answerSlots".
export type {
  ProposalSlot,
  ProposalSlotProps,
  SaveControlSlot,
  SaveControlSlotProps,
} from "./AnswerResult";

interface TurnItemProps {
  readonly turn: Turn;
  readonly nearestQuestion: string;
  readonly onFormSubmitted: (result: PlanResult) => void;
  readonly onAlternativeChosen: (
    turnId: string,
    question: string,
    preferred: string,
    label: string,
  ) => void;
  readonly renderSaveControl?: SaveControlSlot | undefined;
  readonly renderProposal?: ProposalSlot | undefined;
}

/**
 * One turn's own bubble, dispatched on `turn.role` - split out of
 * `TurnList.tsx` so that file stays under eslint's `max-lines` now that it
 * also draws an `alternatives` turn (docs/specs/shortlisting.md, section 4,
 * H5): `question` and `choice` draw their own small bubble here, and an
 * `answer` turn's body is `AnswerResult`'s job, not this file's.
 */
export function TurnItem({
  turn,
  nearestQuestion: question,
  onFormSubmitted,
  onAlternativeChosen,
  renderSaveControl,
  renderProposal,
}: TurnItemProps): JSX.Element {
  if (turn.role === "question") {
    return (
      <Paper
        elevation={0}
        sx={{ ...BUBBLE_SX, alignSelf: "flex-end", p: 2, bgcolor: "action.hover" }}
      >
        <Typography variant="body1">{turn.text}</Typography>
      </Paper>
    );
  }

  if (turn.role === "choice") {
    return (
      <Paper
        elevation={0}
        sx={{ ...BUBBLE_SX, alignSelf: "flex-end", p: 2, bgcolor: "action.hover" }}
      >
        <Typography variant="body1">{turn.label}</Typography>
      </Paper>
    );
  }

  if (turn.role === "alternatives") {
    return (
      <Paper elevation={1} sx={{ p: 2 }}>
        <AlternativesRow
          turn={turn}
          onSelect={(preferred, label) => {
            onAlternativeChosen(turn.id, question, preferred, label);
          }}
        />
      </Paper>
    );
  }

  return (
    <AnswerResult
      result={turn.result}
      originalQuery={question}
      onFormSubmitted={onFormSubmitted}
      renderSaveControl={renderSaveControl}
      renderProposal={renderProposal}
    />
  );
}
