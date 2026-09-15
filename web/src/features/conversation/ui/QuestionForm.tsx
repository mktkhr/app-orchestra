import SendIcon from "@mui/icons-material/Send";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import FormControlLabel from "@mui/material/FormControlLabel";
import Stack from "@mui/material/Stack";
import Switch from "@mui/material/Switch";
import TextField from "@mui/material/TextField";
import { useState, type ChangeEvent, type JSX, type SyntheticEvent } from "react";

interface QuestionFormProps {
  readonly onSubmit: (query: string) => void;
  readonly disabled: boolean;
  /**
   * The 「思考」 switch's current value (platform knobs subproject, decided
   * 2026-09-16): whether the question this form submits, and any chip
   * re-plan started while this value is showing, asks the planner to think
   * before answering. Lives in the conversation store
   * (`model/conversationStore.tsx`), not here, so a chip click
   * (`AlternativesRow`) sends the same value this switch is showing.
   */
  readonly thinking: boolean;
  /** Toggles the switch, persisting the new value (`model/thinkingPreference.ts`). */
  readonly onThinkingChange: (value: boolean) => void;
}

/**
 * The question input, plus the 「思考」 switch (MUI's `Switch`, via
 * `FormControlLabel` for its accessible name - no `src/shared/ui` wrapper:
 * MUI already has both components this needs). A plain MUI form; submitting
 * clears the field but leaves the switch as it was.
 */
export function QuestionForm({
  onSubmit,
  disabled,
  thinking,
  onThinkingChange,
}: QuestionFormProps): JSX.Element {
  const [value, setValue] = useState("");

  const handleChange = (event: ChangeEvent<HTMLInputElement>): void => {
    setValue(event.target.value);
  };

  const handleSubmit = (event: SyntheticEvent<HTMLFormElement>): void => {
    event.preventDefault();
    const trimmed = value.trim();

    if (trimmed === "") {
      return;
    }

    onSubmit(trimmed);
    setValue("");
  };

  const handleThinkingChange = (event: ChangeEvent<HTMLInputElement>): void => {
    onThinkingChange(event.target.checked);
  };

  return (
    <Box component="form" onSubmit={handleSubmit}>
      <Stack direction="row" spacing={1} sx={{ alignItems: "center" }}>
        <TextField
          fullWidth
          label="質問を入力"
          value={value}
          disabled={disabled}
          onChange={handleChange}
        />
        <FormControlLabel
          control={<Switch checked={thinking} onChange={handleThinkingChange} />}
          label="思考"
          disabled={disabled}
        />
        <Button
          type="submit"
          variant="contained"
          endIcon={<SendIcon />}
          // Disabled only while a question is in flight, not because the
          // field is empty. A disabled contained button is a translucent
          // grey rectangle with no edge of its own - make guard-layout
          // measures 1:1 against the page, which is WCAG 1.4.11's way of
          // saying nobody can see it. Submitting an empty question already
          // does nothing (handleSubmit returns early), so there is nothing
          // for the disabled state to prevent.
          disabled={disabled}
          sx={{ whiteSpace: "nowrap" }}
        >
          送信
        </Button>
      </Stack>
    </Box>
  );
}
