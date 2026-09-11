import Alert from "@mui/material/Alert";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import type { JSX } from "react";

import type { Turn } from "../model/turn";

interface TurnListProps {
  readonly turns: readonly Turn[];
}

/** The conversation's turns, in order: a question, then the answer to it. */
export function TurnList({ turns }: TurnListProps): JSX.Element {
  return (
    <Stack spacing={2}>
      {turns.map((turn) => (
        <TurnItem key={turn.id} turn={turn} />
      ))}
    </Stack>
  );
}

function TurnItem({ turn }: { readonly turn: Turn }): JSX.Element {
  if (turn.role === "question") {
    return (
      <Paper elevation={0} sx={{ p: 2, bgcolor: "action.hover" }}>
        <Typography variant="body1">{turn.text}</Typography>
      </Paper>
    );
  }

  return (
    <Paper elevation={1} sx={{ p: 2 }}>
      {/*
       * Task 13-15 plug in here: a `Provenance` header (service / operationId,
       * arguments on expand) plus the component named by turn.result.kind /
       * turn.result.component - `table` (Task 13), `detail` and `form`
       * (Task 14), `choice` (Task 15). Until then this shows only what kind
       * of answer came back, and the message when there is nothing else.
       */}
      <Typography variant="overline" color="text.secondary">
        {turn.result.kind}
      </Typography>
      {turn.result.kind === "none" ? (
        <Typography variant="body1">{turn.result.message}</Typography>
      ) : (
        <Alert severity="info" sx={{ mt: 1 }}>
          この回答（{turn.result.kind}）の表示は後続タスクで実装されます。
        </Alert>
      )}
    </Paper>
  );
}
