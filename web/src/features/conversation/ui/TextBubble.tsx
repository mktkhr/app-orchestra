import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";
import type { JSX, ReactNode } from "react";

import { BUBBLE_SX } from "./bubbleStyles";

interface TextBubbleProps {
  readonly overline: string;
  readonly children: ReactNode;
}

/**
 * The left-aligned, sentence-shaped bubble `AnswerResult` draws for a
 * `kind: "none"` answer, a question-only `kind: "ask"` answer, and its own
 * fallback - three branches that differ only in the overline and what sits
 * under it. Pulled out so each keeps drawing the identical `Paper`/`BUBBLE_SX`
 * shape without `AnswerResult` importing `Paper`, `Typography` and
 * `BUBBLE_SX` a second and third time for the same layout - one more
 * distinct dependency than `harness/quality/eslint`'s
 * `import/max-dependencies` allows that file once it also draws the
 * question-only ask.
 */
export function TextBubble({ overline, children }: TextBubbleProps): JSX.Element {
  return (
    <Paper elevation={1} sx={{ ...BUBBLE_SX, alignSelf: "flex-start", p: 2 }}>
      <Typography variant="overline" color="textSecondary">
        {overline}
      </Typography>
      {children}
    </Paper>
  );
}
