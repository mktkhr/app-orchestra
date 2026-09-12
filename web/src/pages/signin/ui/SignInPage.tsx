import Box from "@mui/material/Box";
import type { JSX } from "react";

import { SignInForm } from "@/features/session";

/**
 * The screen a visitor with no session sees, and nothing else
 * (docs/plans/auth.md Task 4). `AuthGate` (`web/src/app/ui`) is what keeps
 * the chat, the drawer and the bar from rendering behind it.
 */
export function SignInPage(): JSX.Element {
  return (
    <Box
      sx={{
        minHeight: "100vh",
        display: "flex",
        justifyContent: "center",
        alignItems: "center",
        p: 2,
      }}
    >
      <SignInForm />
    </Box>
  );
}
