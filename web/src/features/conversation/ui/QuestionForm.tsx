import SendIcon from "@mui/icons-material/Send";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import { useState, type ChangeEvent, type JSX, type SyntheticEvent } from "react";

interface QuestionFormProps {
  readonly onSubmit: (query: string) => void;
  readonly disabled: boolean;
}

/** The question input. A plain MUI form; submitting clears the field. */
export function QuestionForm({ onSubmit, disabled }: QuestionFormProps): JSX.Element {
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

  return (
    <Box component="form" onSubmit={handleSubmit}>
      <Stack direction="row" spacing={1}>
        <TextField
          fullWidth
          label="質問を入力"
          value={value}
          disabled={disabled}
          onChange={handleChange}
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
