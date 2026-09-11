import Alert from "@mui/material/Alert";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import type { JSX } from "react";

import { Provenance, ResultDetail, ResultForm, ResultTable } from "@/entities/rendering";
import type { PlanResult } from "@/shared/api/client";

import type { Turn } from "../model/turn";

interface TurnListProps {
  readonly turns: readonly Turn[];
  /**
   * Forwarded down to every `ResultForm` this list renders. Defined in
   * `Conversation` (the state owner) and passed down rather than imported,
   * because `ResultForm` lives in `entities/rendering` and cannot reach up
   * into `features/conversation` for the `Turn` type or `setTurns` itself.
   */
  readonly onFormSubmitted: (result: PlanResult) => void;
}

/** The conversation's turns, in order: a question, then the answer to it. */
export function TurnList({ turns, onFormSubmitted }: TurnListProps): JSX.Element {
  return (
    <Stack spacing={2}>
      {turns.map((turn) => (
        <TurnItem key={turn.id} turn={turn} onFormSubmitted={onFormSubmitted} />
      ))}
    </Stack>
  );
}

function TurnItem({
  turn,
  onFormSubmitted,
}: {
  readonly turn: Turn;
  readonly onFormSubmitted: (result: PlanResult) => void;
}): JSX.Element {
  if (turn.role === "question") {
    return (
      <Paper elevation={0} sx={{ p: 2, bgcolor: "action.hover" }}>
        <Typography variant="body1">{turn.text}</Typography>
      </Paper>
    );
  }

  if (turn.result.kind === "none") {
    return (
      <Paper elevation={1} sx={{ p: 2 }}>
        <Typography variant="overline" color="text.secondary">
          {turn.result.kind}
        </Typography>
        <Typography variant="body1">{turn.result.message}</Typography>
      </Paper>
    );
  }

  if (
    turn.result.kind === "result" &&
    turn.result.component === "table" &&
    turn.result.source !== undefined &&
    turn.result.data !== undefined
  ) {
    return (
      <Paper elevation={1} sx={{ p: 2 }}>
        <Provenance source={turn.result.source} />
        <ResultTable data={turn.result.data} fields={turn.result.fields} />
      </Paper>
    );
  }

  if (
    turn.result.kind === "result" &&
    turn.result.component === "detail" &&
    turn.result.data !== undefined
  ) {
    return (
      <Paper elevation={1} sx={{ p: 2 }}>
        {turn.result.source === undefined ? null : <Provenance source={turn.result.source} />}
        <ResultDetail data={turn.result.data} fields={turn.result.fields} />
      </Paper>
    );
  }

  if (
    turn.result.kind === "form" &&
    turn.result.schema !== undefined &&
    turn.result.target !== undefined
  ) {
    return (
      <Paper elevation={1} sx={{ p: 2 }}>
        <ResultForm
          schema={turn.result.schema}
          initial={turn.result.initial}
          target={turn.result.target}
          onSubmitted={onFormSubmitted}
        />
      </Paper>
    );
  }

  // Task 15 plugs in here: `choice` (kind "ask"). Until then this shows only
  // what kind of answer came back.
  return (
    <Paper elevation={1} sx={{ p: 2 }}>
      <Typography variant="overline" color="text.secondary">
        {turn.result.kind}
      </Typography>
      <Alert severity="info" sx={{ mt: 1 }}>
        この回答（{turn.result.kind}）の表示は後続タスクで実装されます。
      </Alert>
    </Paper>
  );
}
