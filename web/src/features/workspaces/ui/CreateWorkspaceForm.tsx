import AddIcon from "@mui/icons-material/Add";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import { useState, type ChangeEvent, type JSX, type SyntheticEvent } from "react";

interface CreateWorkspaceFormProps {
  readonly onCreate: (name: string) => void;
}

/**
 * The drawer's "make a workspace" control. A plain MUI form, styled after
 * `features/conversation/ui/QuestionForm`: the button is never disabled by
 * an empty field - submitting one does nothing (`handleSubmit` returns
 * early) - only ever by the field itself being empty, which a disabled
 * `contained` button cannot show (`make guard-layout`).
 */
export function CreateWorkspaceForm({ onCreate }: CreateWorkspaceFormProps): JSX.Element {
  const [name, setName] = useState("");

  const handleChange = (event: ChangeEvent<HTMLInputElement>): void => {
    setName(event.target.value);
  };

  const handleSubmit = (event: SyntheticEvent<HTMLFormElement>): void => {
    event.preventDefault();
    const trimmed = name.trim();

    if (trimmed === "") {
      return;
    }

    onCreate(trimmed);
    setName("");
  };

  return (
    <Box component="form" onSubmit={handleSubmit} sx={{ px: 2, py: 1 }}>
      <Stack spacing={1}>
        <TextField
          fullWidth
          // No size="small": that renders a 40px input, below the 44px
          // minimum touch target harness/quality/browser/layout.spec.ts
          // measures (make guard-layout).
          label="新しいワークスペース名"
          value={name}
          onChange={handleChange}
        />
        <Button type="submit" variant="outlined" startIcon={<AddIcon />}>
          ワークスペースを作成
        </Button>
      </Stack>
    </Box>
  );
}
