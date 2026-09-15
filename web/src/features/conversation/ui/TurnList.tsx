import Alert from "@mui/material/Alert";
import CircularProgress from "@mui/material/CircularProgress";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import type { JSX } from "react";

import { ResultChoice, ResultForm } from "@/entities/rendering";
import type { PlanResult } from "@/shared/api/client";

import type { Turn } from "../model/turn";
import { renderResultAnswer } from "./renderResultAnswer";
import type { ProposalSlot, SaveControlSlot } from "./renderResultAnswer";

export type { ProposalSlotProps, SaveControlSlotProps } from "./answerSlots";

/**
 * How wide a bubble - a question, or a sentence-shaped answer (C1/C3) - is
 * allowed to get before it wraps. Bounded well short of full width so the
 * alternation (AC-C-101, AC-C-106) reads even when every turn is short; a
 * percentage rather than a fixed pixel count so it still leaves visible
 * margin at 375px (AC-C-106) instead of nearly filling the viewport.
 */
const BUBBLE_MAX_WIDTH = "75%";

/**
 * Shared by every bubble - a question (right), a sentence answer, and the
 * pending spinner (both left, C3/C4) - so the three read as the same shape.
 * `overflowWrap` keeps an unbroken run of characters (a long id, a URL)
 * from stretching the bubble past `BUBBLE_MAX_WIDTH` and pushing the page
 * sideways (AC-C-106); MUI's own word-wrapping handles ordinary text.
 */
const BUBBLE_SX = {
  maxWidth: BUBBLE_MAX_WIDTH,
  overflowWrap: "break-word",
} as const;

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
   * Chosen when a person clicks one of a `result`'s `AlternativesRow`
   * chips - the question that produced it (see `nearestQuestion` below),
   * and the alternative's own `operationId` as `preferred`
   * (docs/specs/shortlisting.md, section 4). `Conversation` re-asks with
   * both, the same way `QuestionForm`'s own submit does.
   */
  readonly onAlternativeChosen: (question: string, preferred: string) => void;
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

/**
 * The text of the nearest `role: "question"` turn before `index`, walking
 * backward from it.
 *
 * A `kind: "ask"` answer carries the planner's own disambiguation question
 * (`PlanResult.question`), not what the person actually typed, and
 * resubmitting `answers` to `/api/plan` needs exactly that original text
 * (`docs/specs/orchestration.md` section 6). Rather than thread a `query`
 * field through `Turn` - every producer of an answer turn would have to
 * remember to set it, including `ResultForm`'s `onSubmitted` path, which has
 * no query to give - this reads it back out of the turn list `Conversation`
 * already keeps. Walking backward instead of just taking `turns[index - 1]`
 * keeps this correct once a choice has already been answered once: the
 * turn right before a second `ask` may be another answer, but the nearest
 * question is still the one to resend.
 */
function nearestQuestion(turns: readonly Turn[], index: number): string {
  for (let cursor = index - 1; cursor >= 0; cursor -= 1) {
    const candidate = turns[cursor];

    if (candidate?.role === "question") {
      return candidate.text;
    }
  }

  return "";
}

interface TurnItemProps {
  readonly turn: Turn;
  readonly nearestQuestion: string;
  readonly onFormSubmitted: (result: PlanResult) => void;
  readonly onAlternativeChosen: (question: string, preferred: string) => void;
  readonly renderSaveControl?: SaveControlSlot | undefined;
  readonly renderProposal?: ProposalSlot | undefined;
}

function TurnItem({
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

  return (
    <AnswerResult
      result={turn.result}
      originalQuery={question}
      onFormSubmitted={onFormSubmitted}
      onAlternativeChosen={onAlternativeChosen}
      renderSaveControl={renderSaveControl}
      renderProposal={renderProposal}
    />
  );
}

interface AnswerResultProps {
  readonly result: PlanResult;
  readonly originalQuery: string;
  readonly onFormSubmitted: (result: PlanResult) => void;
  readonly onAlternativeChosen: (question: string, preferred: string) => void;
  readonly renderSaveControl?: SaveControlSlot | undefined;
  readonly renderProposal?: ProposalSlot | undefined;
}

/**
 * One answer turn's body, dispatched on `result.kind`/`result.component`.
 * Split out of `TurnItem` so each render path - `proposal`, `none`, table,
 * detail, form, choice, and the fallback - stays a small `if`, not one
 * function long enough to trip `max-lines-per-function`.
 */
function AnswerResult({
  result,
  originalQuery,
  onFormSubmitted,
  onAlternativeChosen,
  renderSaveControl,
  renderProposal,
}: AnswerResultProps): JSX.Element | null {
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
    const rendered = renderResultAnswer(
      result,
      originalQuery,
      onAlternativeChosen,
      renderSaveControl,
    );

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
