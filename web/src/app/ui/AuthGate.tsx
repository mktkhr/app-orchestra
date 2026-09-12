import Box from "@mui/material/Box";
import CircularProgress from "@mui/material/CircularProgress";
import type { JSX } from "react";

import { useSession } from "@/features/session";
import { SignInPage } from "@/pages/signin";

import { Shell } from "./Shell";

/**
 * The sign-in gate for the whole application (docs/plans/auth.md Task 4).
 * The chat, the drawer and the bar exist only for someone signed in - not
 * behind the sign-in screen and not for the moment before the startup
 * `GET /api/session` has answered, which is why "still loading" renders
 * neither of them either.
 */
export function AuthGate(): JSX.Element {
  const { status, user } = useSession();

  if (status === "loading") {
    return (
      <Box
        sx={{ minHeight: "100vh", display: "flex", justifyContent: "center", alignItems: "center" }}
      >
        <CircularProgress aria-label="読み込み中" />
      </Box>
    );
  }

  if (user === null) {
    return <SignInPage />;
  }

  return <Shell />;
}
