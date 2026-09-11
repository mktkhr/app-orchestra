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
          disabled={disabled || value.trim() === ""}
          sx={{ whiteSpace: "nowrap" }}
        >
          送信
        </Button>
      </Stack>
    </Box>
  );
}
