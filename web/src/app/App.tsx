import CssBaseline from "@mui/material/CssBaseline";
import { ThemeProvider } from "@mui/material/styles";
import type { JSX } from "react";
import { BrowserRouter } from "react-router";

import { ConversationProvider } from "@/features/conversation";
import { SessionProvider } from "@/features/session";

import { theme } from "./theme";
import { AuthGate } from "./ui/AuthGate";

/**
 * The application: the theme, the router, the session, and whichever of
 * the sign-in screen or the shell the session decides (docs/plans/auth.md
 * Task 4).
 *
 * `BrowserRouter` reads `window.location.pathname` (`docs/specs/routing.md`
 * R1, R3) rather than the hash `useHashRoute` used to. It sits above
 * `AuthGate` rather than below it: signing in is not a route (R4), but
 * `AuthGate` and the sign-in screen it renders live in the same tree as
 * the shell that needs router context, and giving both one router is
 * simpler than deciding which half owns it.
 *
 * The previous DashboardLayout-based shell was replaced with a hand-built
 * MUI shell: that dependency's peer range topped out at @mui/material ^7,
 * this repository is on ^9, and no MUI 8/9 compatible release of it exists.
 */
export function App(): JSX.Element {
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <BrowserRouter>
        <SessionProvider>
          <ConversationProvider>
            <AuthGate />
          </ConversationProvider>
        </SessionProvider>
      </BrowserRouter>
    </ThemeProvider>
  );
}
