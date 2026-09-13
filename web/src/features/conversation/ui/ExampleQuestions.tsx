import Button from "@mui/material/Button";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import type { JSX } from "react";

import { EXAMPLE_QUESTIONS } from "../model/examples";

interface ExampleQuestionsProps {
  readonly onSelect: (query: string) => void;
  readonly disabled: boolean;
}

/** The empty-conversation state: example questions that submit on click. */
export function ExampleQuestions({ onSelect, disabled }: ExampleQuestionsProps): JSX.Element {
  return (
    <Stack spacing={2}>
      <Typography variant="body1" color="textSecondary">
        まだ質問がありません。例えばこんなことを聞けます。
      </Typography>
      <Stack spacing={1} sx={{ alignItems: "flex-start" }}>
        {EXAMPLE_QUESTIONS.map((example) => (
          <Button
            key={example.query}
            variant="outlined"
            disabled={disabled}
            sx={{ justifyContent: "flex-start", textAlign: "left", textTransform: "none" }}
            onClick={() => {
              onSelect(example.query);
            }}
          >
            {example.query}
          </Button>
        ))}
      </Stack>
    </Stack>
  );
}
