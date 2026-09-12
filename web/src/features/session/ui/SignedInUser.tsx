import Button from "@mui/material/Button";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import type { JSX } from "react";

import { useSession } from "../model/sessionContext";

/**
 * The signed-in person's name and the control to sign out
 * (docs/specs/auth.md section 7, "the web shell shows who is signed in").
 * Renders nothing while nobody is signed in - `TopBar` mounts this
 * unconditionally, but `AuthGate` never shows the bar it lives in until
 * `useSession().user` is set, so that case does not arise in practice.
 */
export function SignedInUser(): JSX.Element | null {
  const { user, signOut } = useSession();

  if (user === null) {
    return null;
  }

  const handleSignOut = (): void => {
    void signOut();
  };

  return (
    <Stack direction="row" spacing={2} sx={{ alignItems: "center" }}>
      <Typography component="span">{user.name}</Typography>
      <Button color="inherit" onClick={handleSignOut}>
        サインアウト
      </Button>
    </Stack>
  );
}
