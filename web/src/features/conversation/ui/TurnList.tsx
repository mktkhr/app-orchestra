import Alert from "@mui/material/Alert";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import type { JSX } from "react";

import {
  Provenance,
  RenderedResult,
  ResultChoice,
  ResultForm,
  SaveToWorkspaceControl,
} from "@/entities/rendering";
import type { PlanResult } from "@/shared/api/client";

import type { Turn } from "../model/turn";

interface TurnListProps {
  readonly turns: readonly Turn[];
  /**
   * Forwarded down to every `ResultForm`/`ResultChoice` this list renders.
   * Defined in `Conversation` (the state owner) and passed down rather than
   * imported, because both live in `entities/rendering` and cannot reach up
   * into `features/conversation` for the `Turn` type or `setTurns` itself.
   */
  readonly onFormSubmitted: (result: PlanResult) => void;
}

/** The conversation's turns, in order: a question, then the answer to it. */
export function TurnList({ turns, onFormSubmitted }: TurnListProps): JSX.Element {
  return (
    <Stack spacing={2}>
      {turns.map((turn, index) => (
        <TurnItem
          key={turn.id}
          turn={turn}
          nearestQuestion={nearestQuestion(turns, index)}
          onFormSubmitted={onFormSubmitted}
        />
      ))}
    </Stack>
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
}

function TurnItem({
  turn,
  nearestQuestion: question,
  onFormSubmitted,
}: TurnItemProps): JSX.Element {
  if (turn.role === "question") {
    return (
      <Paper elevation={0} sx={{ p: 2, bgcolor: "action.hover" }}>
        <Typography variant="body1">{turn.text}</Typography>
      </Paper>
    );
  }

  return (
    <AnswerResult result={turn.result} originalQuery={question} onFormSubmitted={onFormSubmitted} />
  );
}

interface AnswerResultProps {
  readonly result: PlanResult;
  readonly originalQuery: string;
  readonly onFormSubmitted: (result: PlanResult) => void;
}

/**
 * One answer turn's body, dispatched on `result.kind`/`result.component`.
 * Split out of `TurnItem` so each render path - `none`, table, detail,
 * form, choice, and the fallback - stays a small `if`, not one function
 * long enough to trip `max-lines-per-function`.
 */
function AnswerResult({ result, originalQuery, onFormSubmitted }: AnswerResultProps): JSX.Element {
  if (result.kind === "none") {
    return (
      <Paper elevation={1} sx={{ p: 2 }}>
        <Typography variant="overline" color="text.secondary">
          {result.kind}
        </Typography>
        <Typography variant="body1">{result.message}</Typography>
      </Paper>
    );
  }

  if (result.kind === "result") {
    const rendered = renderResultAnswer(result, originalQuery);

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
    <Paper elevation={1} sx={{ p: 2 }}>
      <Typography variant="overline" color="text.secondary">
        {result.kind}
      </Typography>
      <Alert severity="info" sx={{ mt: 1 }}>
        この回答（{result.kind}）には表示に必要な情報が含まれていません。
      </Alert>
    </Paper>
  );
}

/**
 * The `kind: "result"` branches of `AnswerResult` - `table` and `detail` -
 * split into their own function so `AnswerResult` stays under
 * `max-lines-per-function`. Returns null for a `component`/`data`
 * combination this deployment's contract allows but that carries none of
 * what either widget needs, so the caller falls through to `AnswerResult`'s
 * own generic fallback instead of this one duplicating it.
 *
 * Each branch also offers `SaveToWorkspaceControl` (AC-W-101) whenever the
 * result carries a `source` to save - a `table` result always does; a
 * `detail` result only sometimes does, the same condition `Provenance`
 * above it already checks.
 */
function renderResultAnswer(result: PlanResult, originalQuery: string): JSX.Element | null {
  if (result.component === "table" && result.source !== undefined && result.data !== undefined) {
    return (
      <Paper elevation={1} sx={{ p: 2 }}>
        <Provenance source={result.source} />
        <RenderedResult component={result.component} data={result.data} fields={result.fields} />
        <SaveToWorkspaceControl
          source={result.source}
          component={result.component}
          defaultTitle={originalQuery}
        />
      </Paper>
    );
  }

  if (result.component === "detail" && result.data !== undefined) {
    return (
      <Paper elevation={1} sx={{ p: 2 }}>
        {result.source === undefined ? null : <Provenance source={result.source} />}
        <RenderedResult component={result.component} data={result.data} fields={result.fields} />
        {result.source === undefined ? null : (
          <SaveToWorkspaceControl
            source={result.source}
            component={result.component}
            defaultTitle={originalQuery}
          />
        )}
      </Paper>
    );
  }

  return null;
}
