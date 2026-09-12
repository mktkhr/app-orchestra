import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import { useState, type ChangeEvent, type JSX, type SyntheticEvent } from "react";

import { useSession } from "../model/sessionContext";

/**
 * Name and password, posted to `POST /api/session` (docs/plans/auth.md
 * Task 4). The submit button stays enabled while the fields are empty - a
 * disabled contained button has no edge `guard-layout` can see - and
 * "signing in" is reported instead by disabling it only while the request
 * is in flight.
 */
export function SignInForm(): JSX.Element {
  const { signIn, signInError } = useSession();
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = (event: SyntheticEvent<HTMLFormElement>): void => {
    event.preventDefault();
    setSubmitting(true);

    void signIn(name, password).finally(() => {
      setSubmitting(false);
    });
  };

  return (
    <Paper
      component="form"
      onSubmit={handleSubmit}
      elevation={3}
      sx={{ p: 4, width: "100%", maxWidth: 360 }}
    >
      <Stack spacing={2}>
        <Typography variant="h5" component="h1">
          サインイン
        </Typography>
        <TextField
          label="名前"
          value={name}
          onChange={(event: ChangeEvent<HTMLInputElement>) => {
            setName(event.target.value);
          }}
        />
        <TextField
          label="パスワード"
          type="password"
          value={password}
          onChange={(event: ChangeEvent<HTMLInputElement>) => {
            setPassword(event.target.value);
          }}
        />
        {signInError === null ? null : <Alert severity="error">{signInError}</Alert>}
        <Button type="submit" variant="contained" disabled={submitting}>
          サインイン
        </Button>
      </Stack>
    </Paper>
  );
}
