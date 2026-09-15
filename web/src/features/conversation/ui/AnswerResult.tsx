import Alert from "@mui/material/Alert";
import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";
import { type JSX, useContext } from "react";

import { ResultChoice, ResultForm } from "@/entities/rendering";
import type { PlanResult } from "@/shared/api/client";

import { ConversationStoreContext } from "../model/conversationContext";
import type { ProposalSlot, SaveControlSlot } from "./answerSlots";
import { BUBBLE_SX } from "./bubbleStyles";
import { renderResultAnswer } from "./renderResultAnswer";

// Re-exported so `TurnList.tsx` can pull both slot types from this module
// instead of also reaching into "./answerSlots" directly - one fewer
// distinct dependency there, which is what oxlint's `import/max-dependencies`
// wants once it also draws an `alternatives` turn. `nearestQuestion` itself
// is a plain function, not a component, so it stays a named export of
// "./renderResultAnswer" rather than moving here: `react/only-export-components`
// only allows a component file (this one) to export components and types.
export type {
  ProposalSlot,
  ProposalSlotProps,
  SaveControlSlot,
  SaveControlSlotProps,
} from "./answerSlots";

interface AnswerResultProps {
  readonly result: PlanResult;
  readonly originalQuery: string;
  readonly onFormSubmitted: (result: PlanResult) => void;
  readonly renderSaveControl?: SaveControlSlot | undefined;
  readonly renderProposal?: ProposalSlot | undefined;
}

/**
 * One answer turn's body, dispatched on `result.kind`/`result.component`.
 * Split out of `TurnList.tsx` so each render path - `proposal`, `none`,
 * table, detail, form, choice, and the fallback - stays a small `if`, not
 * one function long enough to trip `max-lines-per-function`, and so
 * `TurnList.tsx` itself stays under eslint's `max-lines` now that it also
 * draws an `alternatives` turn (docs/specs/shortlisting.md, section 4, H5).
 */
export function AnswerResult({
  result,
  originalQuery,
  onFormSubmitted,
  renderSaveControl,
  renderProposal,
}: AnswerResultProps): JSX.Element | null {
  // The 「思考」 switch lives in the conversation store; an `ask` answered
  // here re-plans through `ResultChoice`, which must post the same value
  // the question was asked with. Outside a provider (a unit test rendering
  // this component alone) the switch reads as off, the platform's default.
  const thinking = useContext(ConversationStoreContext)?.thinking ?? false;

  if (result.kind === "proposal") {
    // No `renderProposal` (the chat screen, N4/AC-N-104) or no `panel`
    // (a deployment whose contract allows a `proposal` with none - not a
    // case the platform ever produces, but this list draws only what it
    // can) draws nothing at all for this turn, rather than a broken
    // control or the generic "missing information" fallback below: a
    // screen this feature offers no proposal on should look like it never
    // came up, not like something failed to render.
    if (result.panel === undefined || renderProposal === undefined) {
      return null;
    }

    return (
      <Paper elevation={1} sx={{ p: 2 }}>
        {renderProposal({ panel: result.panel })}
      </Paper>
    );
  }

  if (result.kind === "none") {
    return (
      <Paper elevation={1} sx={{ ...BUBBLE_SX, alignSelf: "flex-start", p: 2 }}>
        <Typography variant="overline" color="textSecondary">
          {result.kind}
        </Typography>
        <Typography variant="body1">{result.message}</Typography>
      </Paper>
    );
  }

  if (result.kind === "result") {
    const rendered = renderResultAnswer(result, originalQuery, renderSaveControl);

    if (rendered !== null) {
      return rendered;
    }
  }

  if (result.kind === "form" && result.schema !== undefined && result.target !== undefined) {
    return (
      <Paper elevation={1} sx={{ p: 2 }}>
        <ResultForm
          schema={result.schema}
          initial={result.initial}
          target={result.target}
          onSubmitted={onFormSubmitted}
        />
      </Paper>
    );
  }

  if (
    result.kind === "ask" &&
    result.question !== undefined &&
    result.param !== undefined &&
    result.options !== undefined
  ) {
    return (
      <Paper elevation={1} sx={{ p: 2 }}>
        <ResultChoice
          question={result.question}
          param={result.param}
          options={result.options}
          originalQuery={originalQuery}
          thinking={thinking}
          onAnswered={onFormSubmitted}
        />
      </Paper>
    );
  }

  // A response this deployment's contract allows but no branch above
  // matches - a `kind`/`component` combination missing one of its required
  // fields. Shows what kind of answer came back rather than nothing.
  return (
    <Paper elevation={1} sx={{ ...BUBBLE_SX, alignSelf: "flex-start", p: 2 }}>
      <Typography variant="overline" color="textSecondary">
        {result.kind}
      </Typography>
      <Alert severity="info" sx={{ mt: 1 }}>
        この回答（{result.kind}）には表示に必要な情報が含まれていません。
      </Alert>
    </Paper>
  );
}
