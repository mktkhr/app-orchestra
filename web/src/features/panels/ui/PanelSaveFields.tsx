import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import TextField from "@mui/material/TextField";
import type { ChangeEvent, JSX } from "react";

interface PanelSaveFieldsProps {
  readonly title: string;
  readonly onTitleChange: (value: string) => void;
  readonly error: string | null;
  readonly submitting: boolean;
  readonly onSave: () => void;
}

/**
 * Step 5 (`docs/specs/dashboard.md` section 6): the panel's name -
 * defaulted to the operation's own summary, editable - and the control
 * that saves it. Never disabled at rest, only while `submitting`
 * (`make guard-layout` rejects a disabled `contained` button outright), so
 * pressing it with an incomplete form has to say something: `error` then
 * carries `usePanelFields.missingBeforeSave`'s own sentence naming what is
 * still needed, in the same Alert a failed save uses. A button that looks
 * live and does nothing is the defect this avoids.
 */
export function PanelSaveFields({
  title,
  onTitleChange,
  error,
  submitting,
  onSave,
}: PanelSaveFieldsProps): JSX.Element {
  return (
    <>
      <TextField
        fullWidth
        label="パネル名"
        value={title}
        onChange={(event: ChangeEvent<HTMLInputElement>) => {
          onTitleChange(event.target.value);
        }}
      />
      {error === null ? null : <Alert severity="error">{error}</Alert>}
      <Box>
        <Button variant="contained" disabled={submitting} onClick={onSave}>
          追加
        </Button>
      </Box>
    </>
  );
}
