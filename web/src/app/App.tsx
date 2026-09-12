import CssBaseline from "@mui/material/CssBaseline";
import { ThemeProvider } from "@mui/material/styles";
import type { JSX } from "react";

import { ConversationProvider } from "@/features/conversation";
import { SessionProvider } from "@/features/session";

import { theme } from "./theme";
import { AuthGate } from "./ui/AuthGate";

/**
 * The application: the theme, the session, and whichever of the sign-in
 * screen or the shell the session decides (docs/plans/auth.md Task 4).
 *
 * The previous DashboardLayout-based shell was replaced with a hand-built
 * MUI shell: that dependency's peer range topped out at @mui/material ^7,
 * this repository is on ^9, and no MUI 8/9 compatible release of it exists.
 */
export function App(): JSX.Element {
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <SessionProvider>
        <ConversationProvider>
          <AuthGate />
        </ConversationProvider>
      </SessionProvider>
    </ThemeProvider>
  );
}
