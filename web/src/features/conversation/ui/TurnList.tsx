import Alert from "@mui/material/Alert";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import type { JSX } from "react";

import { Provenance, ResultTable } from "@/entities/rendering";

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
        <ResultTable data={turn.result.data} />
      </Paper>
    );
  }

  // Task 14-15 plug in here: `detail` and `form` (kind "result"/"form"), and
  // `choice` (kind "ask"). Until then this shows only what kind of answer
  // came back.
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
